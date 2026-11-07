package dingo

import (
	"fmt"
	"log/slog"
	"reflect"
)

// RequestInjection requests the object to have all fields annotated with `inject` to be filled
func (injector *Injector) RequestInjection(object interface{}) error {
	if injector.stage == INIT {
		injector.delayed = append(injector.delayed, object)
	} else {
		return injector.requestInjection(object, injector.newTrace())
	}

	return nil
}

//nolint:gocognit,cyclop // complexity predates the move, restructuring is out of scope
func (injector *Injector) requestInjection(object interface{}, circularTrace []circularTraceEntry) error {
	if _, ok := object.(reflect.Value); !ok {
		object = reflect.ValueOf(object)
	}

	var (
		injectlist = []reflect.Value{object.(reflect.Value)} //nolint:forcetypeassert // guarded by the checked assertion above
		index      int
		current    reflect.Value
		err        error
	)

	wrapErr := func(err error) error {
		path := current.Type().PkgPath()
		if path == "" {
			if current.Kind() == reflect.Pointer {
				path = current.Elem().Type().PkgPath()
			}
		}

		if path != "" {
			path += "."
		}

		return fmt.Errorf("injecting into %s%s:\n%w", path, current.String(), err)
	}

	for index < len(injectlist) {
		current = injectlist[index]
		ctype := current.Type()

		index++

		if ctype.Kind() != reflect.Pointer && current.MethodByName("Inject").IsValid() {
			return fmt.Errorf("invalid inject receiver %s: %w", current, ErrInvalidInjectReceiver)
		}

		switch ctype.Kind() { //nolint:exhaustive // the kind set is open-ended, default case intended
		// dereference pointer
		case reflect.Pointer:
			if setup := current.MethodByName("Inject"); setup.IsValid() {
				args := make([]reflect.Value, setup.Type().NumIn())
				for arg := range args {
					if args[arg], err = injector.getInstance(setup.Type().In(arg), "", circularTrace); err != nil {
						return wrapErr(err)
					}
				}

				setup.Call(args)
			}

			injectlist = append(injectlist, current.Elem())

		// inject into struct fields
		case reflect.Struct:
			for fieldIndex := 0; fieldIndex < ctype.NumField(); fieldIndex++ {
				//nolint:nestif // nesting predates the move, restructuring is out of scope
				if tag, ok := ctype.Field(fieldIndex).Tag.Lookup("inject"); ok {
					field := current.Field(fieldIndex)
					currentFieldName := ctype.Field(fieldIndex).Name

					if field.Kind() == reflect.Struct {
						return fmt.Errorf("can not inject into struct %#v of %#v", field, current) //nolint:err113 // message text is part of the observed behaviour
					}

					annotation, optional := parseInjectTag(tag)

					instance, err := injector.getInstanceOfTypeWithAnnotation(field.Type(), annotation, nil, optional, circularTrace)
					if err != nil {
						return wrapErr(err)
					}

					if instance.Kind() == reflect.Pointer {
						if instance.Elem().Kind() == reflect.Func || instance.Elem().Kind() == reflect.Interface || instance.Elem().Kind() == reflect.Slice {
							instance = instance.Elem()
						}
					}

					if field.Kind() != reflect.Pointer && field.Kind() != reflect.Interface && instance.Kind() == reflect.Pointer {
						if injector.injectionTracing {
							slog.Info(fmt.Sprintf("SETTING FIELD: %s of type \"%s\"", currentFieldName, ctype.Field(fieldIndex).Type.String()))
						}

						field.Set(instance.Elem())
					} else {
						if field.Kind() == reflect.Pointer && field.Type().Kind() == reflect.Pointer && field.Type().Elem().Kind() == reflect.Interface {
							return wrapErr(fmt.Errorf("field %#v is pointer to interface. %w", currentFieldName, errPointersToInterface))
						}

						if injector.injectionTracing {
							slog.Info(fmt.Sprintf("SETTING FIELD: %s of type \"%s\"", currentFieldName, ctype.Field(fieldIndex).Type.String()))
						}

						field.Set(instance)
					}
				}
			}

		case reflect.Interface:
			if !current.IsNil() {
				injectlist = append(injectlist, current.Elem())
			}

		case reflect.Slice:
			for sliceIndex := 0; sliceIndex < current.Len(); sliceIndex++ {
				injectlist = append(injectlist, current.Index(sliceIndex))
			}

		default:
		}
	}

	return nil
}

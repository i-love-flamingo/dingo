package dingo

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
)

// getInstance creates the new instance of typ, returns a reflect.value
func (injector *Injector) getInstance(typ interface{}, annotatedWith string, circularTrace []circularTraceEntry) (reflect.Value, error) {
	oftype := reflect.TypeOf(typ)

	if oft, ok := typ.(reflect.Type); ok {
		oftype = oft
	} else {
		for oftype.Kind() == reflect.Pointer {
			oftype = oftype.Elem()
		}
	}

	return injector.getInstanceOfTypeWithAnnotation(oftype, annotatedWith, nil, false, circularTrace)
}

func (injector *Injector) findBindingForAnnotatedType(typ reflect.Type, annotation string) *Binding {
	if len(injector.bindings[typ]) > 0 {
		for _, binding := range injector.bindings[typ] {
			if binding.annotatedWith == annotation {
				return binding
			}
		}
	}

	// inject one key of a map-binding
	if key, ok := mapAnnotationKey(annotation); ok {
		return injector.mapbindings[typ][key]
	}

	// ask parent
	if injector.parent != nil {
		return injector.parent.findBindingForAnnotatedType(typ, annotation)
	}

	return nil
}

// getInstanceOfTypeWithAnnotation resolves a requested type, with annotation
//
//nolint:cyclop // complexity predates the move, restructuring is out of scope
func (injector *Injector) getInstanceOfTypeWithAnnotation(typ reflect.Type, annotation string, binding *Binding, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	var (
		final reflect.Value
		err   error
	)

	if typeBinding := injector.findBindingForAnnotatedType(typ, annotation); typeBinding != nil {
		binding = typeBinding
	}

	//nolint:nestif // nesting predates the move, restructuring is out of scope
	if binding != nil {
		if binding.scope != nil {
			if scope, ok := injector.scopes[reflect.TypeOf(binding.scope)]; ok {
				if final, err = scope.ResolveType(typ, annotation, func(requested reflect.Type, annotation string, optional bool) (reflect.Value, error) {
					return injector.createInstanceOfAnnotatedType(requested, annotation, optional, circularTrace)
				}); err != nil {
					return reflect.Value{}, err //nolint:wrapcheck // scope errors pass through unchanged
				}

				if !final.IsValid() {
					return reflect.Value{}, fmt.Errorf("%T did not resolve %s", scope, typ) //nolint:err113 // message text is part of the observed behaviour
				}
			} else {
				return reflect.Value{}, fmt.Errorf("unknown scope %T for %s", binding.scope, typ) //nolint:err113 // message text is part of the observed behaviour
			}
		}
	}

	if !final.IsValid() {
		if final, err = injector.createInstanceOfAnnotatedType(typ, annotation, optional, circularTrace); err != nil {
			return reflect.Value{}, err
		}
	}

	if !final.IsValid() {
		return reflect.Value{}, fmt.Errorf("can not resolve %q", typ.String()) //nolint:err113 // message text is part of the observed behaviour
	}

	return injector.intercept(final, typ, circularTrace)
}

func (injector *Injector) intercept(final reflect.Value, typ reflect.Type, circularTrace []circularTraceEntry) (reflect.Value, error) {
	for _, interceptor := range injector.interceptor[typ] {
		of := final

		final = reflect.New(interceptor)
		if err := injector.requestInjection(final.Interface(), circularTrace); err != nil {
			return reflect.Value{}, err
		}

		final.Elem().Field(0).Set(of)
	}

	if injector.parent != nil {
		return injector.parent.intercept(final, typ, circularTrace)
	}

	return final, nil
}

type unboundError struct {
	binding *Binding
	typ     reflect.Type
}

func (err unboundError) Error() string {
	return fmt.Sprintf("binding is not bound: %v for %s", err.binding, err.typ)
}

func (injector *Injector) resolveBinding(binding *Binding, typ reflect.Type, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	if binding.instance != nil {
		return binding.instance.ivalue, nil
	}

	if binding.provider != nil {
		return binding.provider.create(injector, circularTrace)
	}

	if binding.to != nil {
		if binding.to == typ {
			return reflect.Value{}, fmt.Errorf("circular from %q to %q (annotated with: %q)", typ, binding.to, binding.annotatedWith) //nolint:err113 // message text is part of the observed behaviour
		}

		return injector.getInstanceOfTypeWithAnnotation(binding.to, "", binding, optional, circularTrace)
	}

	return reflect.Value{}, unboundError{binding: binding, typ: typ}
}

// createInstanceOfAnnotatedType resolves a type request with the current injector
//
//nolint:cyclop // complexity predates the move, restructuring is out of scope
func (injector *Injector) createInstanceOfAnnotatedType(typ reflect.Type, annotation string, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	if binding := injector.findBindingForAnnotatedType(typ, annotation); binding != nil {
		r, err := injector.resolveBinding(binding, typ, optional, circularTrace)
		if err == nil || !errors.As(err, new(unboundError)) {
			return r, err
		}

		// todo: proper testcases
		if annotation != "" {
			return injector.getInstanceOfTypeWithAnnotation(binding.typeof, "", binding, false, circularTrace)
		}
	}

	// This for an injection request on a provider, such as `func() MyInstance`
	if isProviderType(typ) && (typ.NumOut() == 1 || typ.NumOut() == 2) {
		return injector.createProvider(typ, annotation, optional, providerCanError(typ), circularTrace), nil
	}

	// This is the injection request for multibindings
	if typ.Kind() == reflect.Slice {
		return injector.resolveMultibinding(typ, annotation, optional, circularTrace)
	}

	// Map Binding injection
	if typ.Kind() == reflect.Map && typ.Key().Kind() == reflect.String {
		return injector.resolveMapbinding(typ, annotation, optional, circularTrace)
	}

	if annotation != "" && !optional {
		return reflect.Value{}, fmt.Errorf("can not automatically create an annotated injection %q with annotation %q", typ, annotation) //nolint:err113 // message text is part of the observed behaviour
	}

	if typ.Kind() == reflect.Interface && !optional {
		return reflect.Value{}, fmt.Errorf("can not instantiate interface %s.%s", typ.PkgPath(), typ.Name()) //nolint:err113 // message text is part of the observed behaviour
	}

	if typ.Kind() == reflect.Func && !optional {
		return reflect.Value{}, fmt.Errorf("can not create a new function %q (Do you want a provider? Then suffix type with Provider)", typ) //nolint:err113 // message text is part of the observed behaviour
	}

	if circularTrace != nil {
		subCircularTrace := startConstructionTrace(circularTrace, typ, annotation)
		defer subCircularTrace[len(subCircularTrace)-1].active.Store(false)

		instance := reflect.New(typ)

		return instance, injector.requestInjection(instance.Interface(), subCircularTrace)
	}

	if injector.injectionTracing {
		if typ.PkgPath() == "" || typ.Name() == "" {
			slog.Info(fmt.Sprintf("INJECTING: %s", typ.String()))
		} else {
			slog.Info(fmt.Sprintf("INJECTING: %s#%s \"%s\"", typ.PkgPath(), typ.Name(), annotation))
		}
	}

	instance := reflect.New(typ)

	return instance, injector.requestInjection(instance.Interface(), nil)
}

func reflectedError(err *error, typ reflect.Type) reflect.Value {
	rerr := reflect.New(errorType).Elem()
	if err == nil || *err == nil {
		return rerr
	}

	rerr.Set(reflect.ValueOf(fmt.Errorf("%q: %w", typ, *err)))

	return rerr
}

//nolint:cyclop // complexity predates the move, restructuring is out of scope
func (injector *Injector) createProvider(typ reflect.Type, annotation string, optional bool, canError bool, circularTrace []circularTraceEntry) reflect.Value {
	return reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		trace := injector.resumeTrace(circularTrace)

		// create a new type
		res := reflect.New(typ.Out(0))
		// dereference possible interface pointer
		if res.Kind() == reflect.Pointer && (res.Elem().Kind() == reflect.Interface || res.Elem().Kind() == reflect.Pointer) {
			res = res.Elem()
		}

		ret := func(v reflect.Value, err error) []reflect.Value {
			switch {
			case err != nil && !canError:
				panic(fmt.Errorf("%q: %w", typ, err))
			case canError:
				return []reflect.Value{v, reflectedError(&err, typ)}
			default:
				return []reflect.Value{v}
			}
		}

		// multibindings
		if res.Elem().Kind() == reflect.Slice {
			return ret(injector.createInstanceOfAnnotatedType(typ.Out(0), annotation, optional, trace))
		}

		// mapbindings
		if res.Elem().Kind() == reflect.Map && res.Elem().Type().Key().Kind() == reflect.String {
			return ret(injector.createInstanceOfAnnotatedType(typ.Out(0), annotation, optional, trace))
		}

		r := ret(injector.getInstance(typ.Out(0), annotation, trace))

		res.Set(r[0])
		r[0] = res

		return r
	})
}

func (injector *Injector) createProviderForBinding(typ reflect.Type, binding *Binding, annotation string, optional bool, canError bool, circularTrace []circularTraceEntry) reflect.Value {
	return reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		trace := injector.resumeTrace(circularTrace)

		// create a new type
		res := reflect.New(binding.typeof)
		// dereference possible interface pointer
		if res.Kind() == reflect.Pointer && (res.Elem().Kind() == reflect.Interface || res.Elem().Kind() == reflect.Pointer) {
			res = res.Elem()
		}

		if r, err := injector.resolveBinding(binding, typ, optional, trace); err == nil {
			res.Set(r)

			if canError {
				return []reflect.Value{res, reflectedError(nil, typ)}
			}

			return []reflect.Value{res}
		}

		// set to actual value
		i, err := injector.getInstance(binding.typeof, annotation, trace)
		if err != nil {
			if canError {
				return []reflect.Value{res, reflectedError(&err, typ)}
			}

			panic(fmt.Errorf("%q: %w", typ, err))
		}

		res.Set(i)
		// return
		if canError {
			return []reflect.Value{res, reflectedError(nil, typ)}
		}

		return []reflect.Value{res}
	})
}

func (injector *Injector) joinMultibindings(typ reflect.Type, annotation string) []*Binding {
	var parent []*Binding
	if injector.parent != nil {
		parent = injector.parent.joinMultibindings(typ, annotation)
	}

	bindings := make([]*Binding, len(parent)+len(injector.multibindings[typ]))
	copy(bindings, parent)
	count := len(parent)

	for _, binding := range injector.multibindings[typ] {
		if binding.annotatedWith == annotation {
			bindings[count] = binding
			count++
		}
	}

	return bindings[:count]
}

func (injector *Injector) resolveMultibinding(typ reflect.Type, annotation string, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	targetType := typ.Elem()
	if targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}

	providerType := targetType
	provider := isProviderType(targetType)
	canError := provider && providerCanError(targetType)

	if provider {
		targetType = targetType.Out(0)
	}

	if bindings := injector.joinMultibindings(targetType, annotation); len(bindings) > 0 {
		slice := reflect.MakeSlice(typ, 0, len(bindings))
		for _, binding := range bindings {
			if provider {
				slice = reflect.Append(slice, injector.createProviderForBinding(providerType, binding, annotation, false, canError, circularTrace))
				continue
			}

			r, err := injector.resolveBinding(binding, typ, optional, circularTrace)
			if err != nil {
				return reflect.Value{}, err
			}

			slice = reflect.Append(slice, r)
		}

		return slice, nil
	}

	return reflect.MakeSlice(typ, 0, 0), nil
}

func (injector *Injector) joinMapbindings(typ reflect.Type, annotation string) map[string]*Binding {
	var parent map[string]*Binding
	if injector.parent != nil {
		parent = injector.parent.joinMapbindings(typ, annotation)
	}

	bindings := make(map[string]*Binding, len(parent)+len(injector.multibindings[typ]))
	for k, v := range parent {
		bindings[k] = v
	}

	for key, binding := range injector.mapbindings[typ] {
		if binding.annotatedWith == annotation {
			bindings[key] = binding
		}
	}

	return bindings
}

func (injector *Injector) resolveMapbinding(typ reflect.Type, annotation string, optional bool, circularTrace []circularTraceEntry) (reflect.Value, error) {
	targetType := typ.Elem()
	if targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}

	providerType := targetType
	provider := isProviderType(targetType)
	canError := provider && providerCanError(targetType)

	if provider {
		targetType = targetType.Out(0)
	}

	if bindings := injector.joinMapbindings(targetType, annotation); len(bindings) > 0 {
		result := reflect.MakeMapWithSize(typ, len(bindings))
		for key, binding := range bindings {
			if provider {
				result.SetMapIndex(reflect.ValueOf(key), injector.createProviderForBinding(providerType, binding, annotation, false, canError, circularTrace))
				continue
			}

			r, err := injector.resolveBinding(binding, typ, optional, circularTrace)
			if err != nil {
				return reflect.Value{}, err
			}

			result.SetMapIndex(reflect.ValueOf(key), r)
		}

		return result, nil
	}

	return reflect.MakeMap(typ), nil
}

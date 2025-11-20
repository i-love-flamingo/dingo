package dingo

import (
	"fmt"
	"reflect"
)

// BindMulti creates a new type-safe multi-binding for type T.
// Multi-bindings allow multiple implementations to be registered and injected as a slice.
//
// Returns *Binding for compatibility with the existing API, allowing method chaining.
//
// FAIL FAST: Performs validation at binding time to ensure proper configuration.
//
// Example:
//
//	// Register multiple plugins
//	BindMulti[Plugin](injector).To(PluginA{})
//	BindMulti[Plugin](injector).To(PluginB{})
//
//	// Inject as slice
//	type Service struct {
//	    Plugins []Plugin `inject:""`
//	}
//
//	// Or retrieve programmatically
//	plugins, _ := GetInstance[[]Plugin](injector)
func BindMulti[T any](injector *Injector) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate that we're not binding nil
	if bindtype == nil {
		panic("cannot create multi-binding for nil type")
	}

	binding := &Binding{
		typeof: bindtype,
	}

	// Add to injector's multi-bindings
	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb

	return binding
}

// BindMultiTo creates a type-safe multi-binding from interface F to concrete type T.
// This validates at binding time that T is assignable to F.
//
// FAIL FAST: Runtime validation of assignability at binding time.
//
// Example:
//
//	BindMultiTo[Plugin, *PluginImpl](injector).AnnotatedWith("production")
func BindMultiTo[F, T any](injector *Injector) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}

	fromType := reflect.TypeOf((*F)(nil)).Elem()
	toType := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate types
	if fromType == nil {
		panic("cannot create multi-binding from nil type")
	}
	if toType == nil {
		panic("cannot create multi-binding to nil type")
	}

	// Handle pointer types
	actualToType := toType
	for actualToType.Kind() == reflect.Ptr {
		actualToType = actualToType.Elem()
	}

	// FAIL FAST: Validate assignability at binding time
	if !actualToType.AssignableTo(fromType) && !reflect.PtrTo(actualToType).AssignableTo(fromType) {
		panic(fmt.Sprintf(
			"multi-binding validation failed: %s#%s is not assignable to %s#%s",
			actualToType.PkgPath(), actualToType.Name(),
			fromType.PkgPath(), fromType.Name(),
		))
	}

	binding := &Binding{
		typeof: fromType,
		to:     actualToType,
	}

	// Add to injector's multi-bindings
	imb := injector.multibindings[fromType]
	imb = append(imb, binding)
	injector.multibindings[fromType] = imb

	return binding
}

// BindMultiInstance creates a multi-binding to a specific instance with type safety.
//
// FAIL FAST: Validates instance type is assignable to T at binding time.
//
// Example:
//
//	plugin := &MyPlugin{configured: true}
//	BindMultiInstance[Plugin](injector, plugin)
func BindMultiInstance[T any](injector *Injector, instance T) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	instanceType := reflect.TypeOf(instance)
	instanceValue := reflect.ValueOf(instance)

	// FAIL FAST: Validate types
	if bindtype == nil {
		panic("cannot create multi-binding for nil type")
	}
	if instanceType == nil {
		panic("cannot bind multi-binding to nil instance")
	}

	// FAIL FAST: Validate assignability
	if !instanceType.AssignableTo(bindtype) && !instanceType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"multi-binding validation failed: instance of type %s#%s is not assignable to %s#%s",
			instanceType.PkgPath(), instanceType.Name(),
			bindtype.PkgPath(), bindtype.Name(),
		))
	}

	binding := &Binding{
		typeof: bindtype,
		instance: &Instance{
			itype:  instanceType,
			ivalue: instanceValue,
		},
	}

	// Add to injector's multi-bindings
	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb

	return binding
}

// BindMultiProvider creates a multi-binding to a simple provider function with type safety.
// The provider takes no arguments and returns T.
//
// For providers that need dependencies injected, use BindMultiProviderFunc.
//
// Example:
//
//	BindMultiProvider[Plugin](injector, func() Plugin {
//	    return &DynamicPlugin{timestamp: time.Now()}
//	})
func BindMultiProvider[T any](injector *Injector, provider func() T) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}
	if provider == nil {
		panic("cannot bind multi-binding to nil provider")
	}

	return BindMultiProviderFunc[T](injector, provider)
}

// BindMultiProviderWithError creates a multi-binding to a provider that can return errors.
//
// Example:
//
//	BindMultiProviderWithError[Plugin](injector, func() (Plugin, error) {
//	    return loadPlugin()
//	})
func BindMultiProviderWithError[T any](injector *Injector, provider func() (T, error)) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}
	if provider == nil {
		panic("cannot bind multi-binding to nil provider")
	}

	return BindMultiProviderFunc[T](injector, provider)
}

// BindMultiProviderFunc creates a multi-binding to a provider function with automatic dependency injection.
// The provider can have any parameters, which will be automatically resolved by the injector.
//
// FAIL FAST: Comprehensive validation at binding time.
//
// Example:
//
//	BindMultiProviderFunc[Handler](injector, func(logger Logger) Handler {
//	    return &LoggingHandler{logger: logger}
//	})
func BindMultiProviderFunc[T any](injector *Injector, providerFunc interface{}) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}
	if providerFunc == nil {
		panic("cannot bind multi-binding to nil provider")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	if bindtype == nil {
		panic("cannot create multi-binding for nil type")
	}

	fnValue := reflect.ValueOf(providerFunc)
	fnType := fnValue.Type()

	// FAIL FAST: Validate provider is a function
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf(
			"multi-binding validation failed: provider must be a function, got %s",
			fnType.Kind(),
		))
	}

	// FAIL FAST: Validate provider has at least one return value
	if fnType.NumOut() == 0 {
		panic("multi-binding validation failed: provider function must return at least one value")
	}

	// FAIL FAST: Validate provider has at most two return values
	if fnType.NumOut() > 2 {
		panic(fmt.Sprintf(
			"multi-binding validation failed: provider function must return at most 2 values (T or (T, error)), got %d",
			fnType.NumOut(),
		))
	}

	returnType := fnType.Out(0)

	// FAIL FAST: Validate second return value is error type if present
	if fnType.NumOut() == 2 {
		errorInterface := reflect.TypeOf((*error)(nil)).Elem()
		if !fnType.Out(1).AssignableTo(errorInterface) {
			panic(fmt.Sprintf(
				"multi-binding validation failed: second return value must be error, got %s",
				fnType.Out(1),
			))
		}
	}

	// FAIL FAST: Validate return type is assignable to bound type
	if !returnType.AssignableTo(bindtype) && !returnType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"multi-binding validation failed: provider returns %q which is not assignable to %q",
			returnType,
			bindtype,
		))
	}

	binding := &Binding{
		typeof: bindtype,
		provider: &Provider{
			fnctype: returnType,
			fnc:     fnValue,
			binding: nil, // will be set after
		},
	}
	binding.provider.binding = binding

	// Add to injector's multi-bindings
	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb

	return binding
}

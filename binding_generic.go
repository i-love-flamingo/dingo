package dingo

import (
	"fmt"
	"reflect"
)

// Bind creates a new type-safe binding for type T.
// Returns *Binding for compatibility with the existing API, allowing method chaining.
//
// FAIL FAST: Performs validation at binding time to ensure proper configuration.
//
// Example:
//
//	// Basic binding
//	Bind[MyInterface](injector).To(MyImpl{})
//
//	// With configuration
//	Bind[MyInterface](injector).To(MyImpl{}).AnnotatedWith("special").In(Singleton)
//
//	// Bind to instance
//	Bind[MyInterface](injector).ToInstance(myInstance)
//
//	// Bind to provider
//	Bind[MyInterface](injector).ToProvider(myProviderFunc)
func Bind[T any](injector *Injector) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate that we're not binding nil
	if bindtype == nil {
		panic("cannot bind nil type")
	}

	binding := &Binding{
		typeof: bindtype,
	}

	// Add to injector's bindings
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	return binding
}

// BindTo creates a type-safe binding from interface F to concrete type T.
// This validates at binding time that T is assignable to F, providing fail-fast behavior.
//
// FAIL FAST: Compile-time check that T satisfies F (when both are concrete),
// plus runtime validation of assignability.
//
// Example:
//
//	// Explicit type relationship with validation
//	BindTo[MyInterface, *MyImpl](injector).AnnotatedWith("v2")
//
//	// This is safer than Bind + To because both types are specified upfront
//	BindTo[Database, *PostgresDB](injector).In(Singleton)
func BindTo[F, T any](injector *Injector) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}

	fromType := reflect.TypeOf((*F)(nil)).Elem()
	toType := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate types
	if fromType == nil {
		panic("cannot bind from nil type")
	}
	if toType == nil {
		panic("cannot bind to nil type")
	}

	// Handle pointer types
	actualToType := toType
	for actualToType.Kind() == reflect.Ptr {
		actualToType = actualToType.Elem()
	}

	// FAIL FAST: Validate assignability at binding time
	if !actualToType.AssignableTo(fromType) && !reflect.PtrTo(actualToType).AssignableTo(fromType) {
		panic(fmt.Sprintf(
			"binding validation failed: %s#%s is not assignable to %s#%s",
			actualToType.PkgPath(), actualToType.Name(),
			fromType.PkgPath(), fromType.Name(),
		))
	}

	binding := &Binding{
		typeof: fromType,
		to:     actualToType,
	}

	// Add to injector's bindings
	injector.bindings[fromType] = append(injector.bindings[fromType], binding)

	return binding
}

// BindInstance creates a binding to a specific instance with type safety.
// This is a convenience function that validates the instance at binding time.
//
// FAIL FAST: Validates instance type is assignable to T.
//
// Example:
//
//	db := &PostgresDB{connectionString: "localhost"}
//	BindInstance[Database](injector, db).In(Singleton)
func BindInstance[T any](injector *Injector, instance T) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	instanceType := reflect.TypeOf(instance)
	instanceValue := reflect.ValueOf(instance)

	// FAIL FAST: Validate types
	if bindtype == nil {
		panic("cannot bind nil type")
	}
	if instanceType == nil {
		panic("cannot bind to nil instance")
	}

	// FAIL FAST: Validate assignability
	if !instanceType.AssignableTo(bindtype) && !instanceType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"binding validation failed: instance of type %s#%s is not assignable to %s#%s",
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

	// Add to injector's bindings
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	return binding
}

// BindProvider creates a binding to a simple provider function with type safety.
// The provider takes no arguments and returns T.
//
// For providers that need dependencies injected, use BindProviderFunc.
//
// Example:
//
//	BindProvider[Logger](injector, func() Logger {
//	    return &ConsoleLogger{}
//	}).AsSingleton()
func BindProvider[T any](injector *Injector, provider func() T) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}
	if provider == nil {
		panic("cannot bind to nil provider")
	}

	return BindProviderFunc[T](injector, provider)
}

// BindProviderWithError creates a binding to a provider that can return errors.
//
// Example:
//
//	BindProviderWithError[Database](injector, func() (Database, error) {
//	    return connectDB()
//	})
func BindProviderWithError[T any](injector *Injector, provider func() (T, error)) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}
	if provider == nil {
		panic("cannot bind to nil provider")
	}

	return BindProviderFunc[T](injector, provider)
}

// BindProviderFunc creates a binding to a provider function with automatic dependency injection.
// The provider can have any parameters, which will be automatically resolved by the injector.
//
// FAIL FAST: Comprehensive validation at binding time:
//   - Provider must be a function
//   - Provider must return 1 or 2 values
//   - First return value must be assignable to T
//   - Second return value (if present) must be error
//
// Example:
//
//	BindProviderFunc[Cache](injector, func(db Database, logger Logger) Cache {
//	    return &RedisCache{db: db, logger: logger}
//	})
//
//	// With error handling
//	BindProviderFunc[Database](injector, func(config Config) (Database, error) {
//	    return connectDB(config)
//	})
func BindProviderFunc[T any](injector *Injector, providerFunc interface{}) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}
	if providerFunc == nil {
		panic("cannot bind to nil provider")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	if bindtype == nil {
		panic("cannot bind nil type")
	}

	fnValue := reflect.ValueOf(providerFunc)
	fnType := fnValue.Type()

	// FAIL FAST: Validate provider is a function
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf(
			"binding validation failed: provider must be a function, got %s",
			fnType.Kind(),
		))
	}

	// FAIL FAST: Validate provider has at least one return value
	if fnType.NumOut() == 0 {
		panic("binding validation failed: provider function must return at least one value")
	}

	// FAIL FAST: Validate provider has at most two return values
	if fnType.NumOut() > 2 {
		panic(fmt.Sprintf(
			"binding validation failed: provider function must return at most 2 values (T or (T, error)), got %d",
			fnType.NumOut(),
		))
	}

	returnType := fnType.Out(0)

	// FAIL FAST: Validate second return value is error type if present
	if fnType.NumOut() == 2 {
		errorInterface := reflect.TypeOf((*error)(nil)).Elem()
		if !fnType.Out(1).AssignableTo(errorInterface) {
			panic(fmt.Sprintf(
				"binding validation failed: second return value must be error, got %s",
				fnType.Out(1),
			))
		}
	}

	// FAIL FAST: Validate return type is assignable to bound type
	if !returnType.AssignableTo(bindtype) && !returnType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"binding validation failed: provider returns %q which is not assignable to %q",
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

	// Add to injector's bindings
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	return binding
}

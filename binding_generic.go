package dingo

import (
	"fmt"
	"reflect"
)

// BindingOption is a functional option for configuring bindings.
// This follows the Go idiom of functional options for clean, extensible configuration.
//
// Example:
//
//	Bind[Logger, *ConsoleLogger](injector, WithAnnotation("console"), AsSingleton())
type BindingOption func(*Binding)

// WithAnnotation sets the binding's annotation name.
// Annotations allow multiple bindings of the same type to coexist.
//
// Example:
//
//	Bind[Logger, *ConsoleLogger](injector, WithAnnotation("console"))
//	Bind[Logger, *FileLogger](injector, WithAnnotation("file"))
func WithAnnotation(annotation string) BindingOption {
	return func(b *Binding) {
		if annotation == "" {
			panic("binding validation failed: annotation cannot be empty string")
		}
		b.annotatedWith = annotation
	}
}

// WithScope sets the binding's scope.
// Common scopes are Singleton and ChildSingleton.
//
// Example:
//
//	Bind[Database, *PostgresDB](injector, WithScope(Singleton))
func WithScope(scope Scope) BindingOption {
	return func(b *Binding) {
		if scope == nil {
			panic("binding validation failed: scope cannot be nil")
		}
		b.scope = scope
	}
}

// AsSingleton is a convenience option that sets the scope to Singleton.
// Singletons are shared across the entire application.
//
// Example:
//
//	Bind[Database, *PostgresDB](injector, AsSingleton())
func AsSingleton() BindingOption {
	return WithScope(Singleton)
}

// AsChildSingleton is a convenience option that sets the scope to ChildSingleton.
// Child singletons are shared within a child injector but not across parent/child boundaries.
//
// Example:
//
//	Bind[RequestContext, *Context](injector, AsChildSingleton())
func AsChildSingleton() BindingOption {
	return WithScope(ChildSingleton)
}

// AsEagerSingleton marks the binding as an eager singleton.
// Eager singletons are instantiated immediately when the injector is initialized.
//
// Example:
//
//	Bind[AppInitializer, *Initializer](injector, AsEagerSingleton())
func AsEagerSingleton() BindingOption {
	return func(b *Binding) {
		b.scope = Singleton
		b.eager = true
	}
}

// Bind creates a type-safe binding from interface/type F to concrete type T.
// This is the main entry point for the generic binding API, combining both the
// "what to bind" and "what to bind to" in a single function call.
//
// FAIL FAST: Performs comprehensive validation at binding time to ensure:
//   - T is assignable to F
//   - All options are valid
//
// Example:
//
//	// Basic binding
//	Bind[Logger, *ConsoleLogger](injector)
//
//	// With options
//	Bind[Logger, *ConsoleLogger](injector, WithAnnotation("console"), AsSingleton())
//
//	// Database with eager singleton
//	Bind[Database, *PostgresDB](injector, AsEagerSingleton())
func Bind[F, T any](injector *Injector, opts ...BindingOption) *Binding {
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

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
	}

	// Add to injector's bindings
	injector.bindings[fromType] = append(injector.bindings[fromType], binding)

	return binding
}

// BindInstance creates a binding to a specific instance with type safety.
// The type T is inferred from the instance parameter.
//
// FAIL FAST: Validates instance type at binding time.
//
// Example:
//
//	db := &PostgresDB{connectionString: "localhost"}
//	BindInstance[Database](injector, db, AsSingleton())
//
//	logger := &FileLogger{path: "/var/log/app.log"}
//	BindInstance[Logger](injector, logger, WithAnnotation("file"))
func BindInstance[T any](injector *Injector, instance T, opts ...BindingOption) *Binding {
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

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
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
//	}, AsSingleton())
func BindProvider[T any](injector *Injector, provider func() T, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}
	if provider == nil {
		panic("cannot bind to nil provider")
	}

	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProviderWithError creates a binding to a provider that can return errors.
//
// Example:
//
//	BindProviderWithError[Database](injector, func() (Database, error) {
//	    return connectDB()
//	}, AsSingleton())
func BindProviderWithError[T any](injector *Injector, provider func() (T, error), opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot bind on nil injector")
	}
	if provider == nil {
		panic("cannot bind to nil provider")
	}

	return BindProviderFunc[T](injector, provider, opts...)
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
//	}, AsSingleton())
func BindProviderFunc[T any](injector *Injector, providerFunc interface{}, opts ...BindingOption) *Binding {
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

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
	}

	// Add to injector's bindings
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	return binding
}

package dingo

// This file provides a Go-idiomatic generic binding API for dingo using type parameters.
//
// The API uses the functional options pattern for configuration and combines "from" and "to"
// types in single function calls, eliminating the need for method chaining.
//
// Key features:
// - Type-safe bindings using Go generics
// - Functional options pattern for clean configuration
// - Combined Bind+To in single function call
// - Fail-fast validation at binding time (not resolution time)
// - Full compatibility with existing reflection-based API
//
// Example:
//
//	Bind[Logger, *ConsoleLogger](injector, WithAnnotation("console"), AsSingleton())

import (
	"fmt"
	"reflect"
)

// ============================================================================
// Type-Safe Provider Function Types
// ============================================================================

// Provider0 is a type-safe provider function with no dependencies that returns T.
// Using this instead of interface{} provides compile-time type safety.
type Provider0[T any] func() T

// Provider1 is a type-safe provider function with 1 dependency.
type Provider1[T, D1 any] func(D1) T

// Provider2 is a type-safe provider function with 2 dependencies.
type Provider2[T, D1, D2 any] func(D1, D2) T

// Provider3 is a type-safe provider function with 3 dependencies.
type Provider3[T, D1, D2, D3 any] func(D1, D2, D3) T

// Provider4 is a type-safe provider function with 4 dependencies.
type Provider4[T, D1, D2, D3, D4 any] func(D1, D2, D3, D4) T

// Provider5 is a type-safe provider function with 5 dependencies.
type Provider5[T, D1, D2, D3, D4, D5 any] func(D1, D2, D3, D4, D5) T

// ProviderWithError0 is a type-safe provider that returns T and error, with no dependencies.
type ProviderWithError0[T any] func() (T, error)

// ProviderWithError1 is a type-safe provider that returns T and error, with 1 dependency.
type ProviderWithError1[T, D1 any] func(D1) (T, error)

// ProviderWithError2 is a type-safe provider that returns T and error, with 2 dependencies.
type ProviderWithError2[T, D1, D2 any] func(D1, D2) (T, error)

// ProviderWithError3 is a type-safe provider that returns T and error, with 3 dependencies.
type ProviderWithError3[T, D1, D2, D3 any] func(D1, D2, D3) (T, error)

// ProviderWithError4 is a type-safe provider that returns T and error, with 4 dependencies.
type ProviderWithError4[T, D1, D2, D3, D4 any] func(D1, D2, D3, D4) (T, error)

// ProviderWithError5 is a type-safe provider that returns T and error, with 5 dependencies.
type ProviderWithError5[T, D1, D2, D3, D4, D5 any] func(D1, D2, D3, D4, D5) (T, error)

// ============================================================================
// Type Constraints and Verification
// ============================================================================

// Implements is a runtime verification helper that T implements I.
// This uses reflection to verify interface implementation and panics if not satisfied.
//
// Note: For true compile-time verification, use the traditional Go pattern:
//
//	var _ MyInterface = (*MyType)(nil)
//
// Example:
//
//	type MyImpl struct{}
//	func (m *MyImpl) DoSomething() {}
//
//	// Verify implementation (panics if not satisfied)
//	_ = Implements[MyInterface, MyImpl]()
func Implements[I, T any]() struct{} {
	// Verify at runtime using reflection
	var t T
	var i *I
	tType := reflect.TypeOf(t)
	iType := reflect.TypeOf(i).Elem()

	if tType != nil && iType != nil {
		if !tType.Implements(iType) && !reflect.PtrTo(tType).Implements(iType) {
			panic(fmt.Sprintf(
				"type verification failed: %s does not implement %s",
				tType, iType,
			))
		}
	}

	return struct{}{}
}

// MustImplement is a runtime verification that T implements I, with a clear panic message.
// Use this in init() functions or at binding time for additional safety.
//
// Example:
//
//	func init() {
//	    MustImplement[MyInterface, *MyImpl]()
//	}
func MustImplement[I, T any]() {
	var t T
	tType := reflect.TypeOf(t)
	iType := reflect.TypeOf((*I)(nil)).Elem()

	if tType == nil || iType == nil {
		panic(fmt.Sprintf("MustImplement: cannot verify nil types"))
	}

	if !tType.Implements(iType) && !reflect.PtrTo(tType).Implements(iType) {
		panic(fmt.Sprintf(
			"type safety violation: %s does not implement %s",
			tType, iType,
		))
	}
}

// ============================================================================
// Functional Options
// ============================================================================

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

// ============================================================================
// Core Binding Functions
// ============================================================================

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

// ============================================================================
// Type-Safe Provider Binding Functions
// ============================================================================
// These functions use the Provider* and ProviderWithError* types for compile-time
// type safety, eliminating the need for interface{} and runtime reflection validation
// of provider signatures.

// BindProvider1 creates a binding using a type-safe provider with 1 dependency.
// This provides compile-time type safety for the provider signature.
//
// Example:
//
//	BindProvider1(injector, func(db Database) Cache {
//	    return &RedisCache{db: db}
//	}, AsSingleton())
func BindProvider1[T, D1 any](injector *Injector, provider Provider1[T, D1], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProvider2 creates a binding using a type-safe provider with 2 dependencies.
//
// Example:
//
//	BindProvider2(injector, func(db Database, logger Logger) Cache {
//	    return &RedisCache{db: db, logger: logger}
//	}, AsSingleton())
func BindProvider2[T, D1, D2 any](injector *Injector, provider Provider2[T, D1, D2], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProvider3 creates a binding using a type-safe provider with 3 dependencies.
func BindProvider3[T, D1, D2, D3 any](injector *Injector, provider Provider3[T, D1, D2, D3], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProvider4 creates a binding using a type-safe provider with 4 dependencies.
func BindProvider4[T, D1, D2, D3, D4 any](injector *Injector, provider Provider4[T, D1, D2, D3, D4], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProvider5 creates a binding using a type-safe provider with 5 dependencies.
func BindProvider5[T, D1, D2, D3, D4, D5 any](injector *Injector, provider Provider5[T, D1, D2, D3, D4, D5], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProviderWithError1 creates a binding using a type-safe provider with 1 dependency that can return an error.
//
// Example:
//
//	BindProviderWithError1(injector, func(config Config) (Database, error) {
//	    return connectDB(config)
//	}, AsSingleton())
func BindProviderWithError1[T, D1 any](injector *Injector, provider ProviderWithError1[T, D1], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProviderWithError2 creates a binding using a type-safe provider with 2 dependencies that can return an error.
func BindProviderWithError2[T, D1, D2 any](injector *Injector, provider ProviderWithError2[T, D1, D2], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProviderWithError3 creates a binding using a type-safe provider with 3 dependencies that can return an error.
func BindProviderWithError3[T, D1, D2, D3 any](injector *Injector, provider ProviderWithError3[T, D1, D2, D3], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProviderWithError4 creates a binding using a type-safe provider with 4 dependencies that can return an error.
func BindProviderWithError4[T, D1, D2, D3, D4 any](injector *Injector, provider ProviderWithError4[T, D1, D2, D3, D4], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// BindProviderWithError5 creates a binding using a type-safe provider with 5 dependencies that can return an error.
func BindProviderWithError5[T, D1, D2, D3, D4, D5 any](injector *Injector, provider ProviderWithError5[T, D1, D2, D3, D4, D5], opts ...BindingOption) *Binding {
	return BindProviderFunc[T](injector, provider, opts...)
}

// ============================================================================
// Multi-Binding Functions
// ============================================================================

// BindMulti creates a type-safe multi-binding from interface/type F to concrete type T.
// Multi-bindings allow multiple implementations to be registered and injected as a slice.
//
// FAIL FAST: Performs comprehensive validation at binding time.
//
// Example:
//
//	// Register multiple plugins
//	BindMulti[Plugin, *PluginA](injector)
//	BindMulti[Plugin, *PluginB](injector)
//	BindMulti[Plugin, *PluginC](injector, WithAnnotation("optional"))
//
//	// Inject as slice
//	type Service struct {
//	    Plugins []Plugin `inject:""`
//	}
//
//	// Or retrieve programmatically
//	plugins, _ := GetInstance[[]Plugin](injector)
func BindMulti[F, T any](injector *Injector, opts ...BindingOption) *Binding {
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

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
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
//
//	logger := &FileLogger{path: "/var/log/app.log"}
//	BindMultiInstance[Logger](injector, logger, WithAnnotation("production"))
func BindMultiInstance[T any](injector *Injector, instance T, opts ...BindingOption) *Binding {
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

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
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
func BindMultiProvider[T any](injector *Injector, provider func() T, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}
	if provider == nil {
		panic("cannot bind multi-binding to nil provider")
	}

	return BindMultiProviderFunc[T](injector, provider, opts...)
}

// BindMultiProviderWithError creates a multi-binding to a provider that can return errors.
//
// Example:
//
//	BindMultiProviderWithError[Plugin](injector, func() (Plugin, error) {
//	    return loadPlugin()
//	})
func BindMultiProviderWithError[T any](injector *Injector, provider func() (T, error), opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create multi-binding on nil injector")
	}
	if provider == nil {
		panic("cannot bind multi-binding to nil provider")
	}

	return BindMultiProviderFunc[T](injector, provider, opts...)
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
//
//	BindMultiProviderFunc[Middleware](injector, func(auth Auth) Middleware {
//	    return &AuthMiddleware{auth: auth}
//	}, WithAnnotation("security"))
func BindMultiProviderFunc[T any](injector *Injector, providerFunc interface{}, opts ...BindingOption) *Binding {
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

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
	}

	// Add to injector's multi-bindings
	imb := injector.multibindings[bindtype]
	imb = append(imb, binding)
	injector.multibindings[bindtype] = imb

	return binding
}

// ============================================================================
// Map-Binding Functions
// ============================================================================

// BindMap creates a type-safe map-binding from interface/type F to concrete type T with a specified key.
// Map-bindings allow multiple implementations to be registered with string keys
// and injected as a map[string]T or individually by key.
//
// FAIL FAST: Performs comprehensive validation at binding time.
//
// Example:
//
//	// Register multiple storage backends
//	BindMap[Storage, *RedisStorage](injector, "redis")
//	BindMap[Storage, *PostgresStorage](injector, "postgres")
//	BindMap[Storage, *S3Storage](injector, "s3", WithAnnotation("cloud"))
//
//	// Inject as map
//	type Service struct {
//	    Storages map[string]Storage `inject:""`
//	}
//
//	// Or inject individual backend
//	type RedisService struct {
//	    Storage Storage `inject:"map:redis"`
//	}
func BindMap[F, T any](injector *Injector, key string, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create map-binding on nil injector")
	}

	// FAIL FAST: Validate key is not empty
	if key == "" {
		panic("map-binding validation failed: key cannot be empty string")
	}

	fromType := reflect.TypeOf((*F)(nil)).Elem()
	toType := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate types
	if fromType == nil {
		panic("cannot create map-binding from nil type")
	}
	if toType == nil {
		panic("cannot create map-binding to nil type")
	}

	// Handle pointer types
	actualToType := toType
	for actualToType.Kind() == reflect.Ptr {
		actualToType = actualToType.Elem()
	}

	// FAIL FAST: Validate assignability at binding time
	if !actualToType.AssignableTo(fromType) && !reflect.PtrTo(actualToType).AssignableTo(fromType) {
		panic(fmt.Sprintf(
			"map-binding validation failed (key=%q): %s#%s is not assignable to %s#%s",
			key,
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

	// Initialize map if needed
	bindingMap := injector.mapbindings[fromType]
	if bindingMap == nil {
		bindingMap = make(map[string]*Binding)
	}

	// Store the binding with the key
	bindingMap[key] = binding
	injector.mapbindings[fromType] = bindingMap

	return binding
}

// BindMapInstance creates a map-binding to a specific instance with type safety.
//
// FAIL FAST: Validates instance type is assignable to T at binding time.
//
// Example:
//
//	cache := &RedisCache{configured: true}
//	BindMapInstance[Cache](injector, "redis", cache)
//
//	db := &PostgresDB{connectionString: "localhost"}
//	BindMapInstance[Database](injector, "primary", db, AsSingleton())
func BindMapInstance[T any](injector *Injector, key string, instance T, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create map-binding on nil injector")
	}

	// FAIL FAST: Validate key is not empty
	if key == "" {
		panic("map-binding validation failed: key cannot be empty string")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	instanceType := reflect.TypeOf(instance)
	instanceValue := reflect.ValueOf(instance)

	// FAIL FAST: Validate types
	if bindtype == nil {
		panic("cannot create map-binding for nil type")
	}
	if instanceType == nil {
		panic(fmt.Sprintf("cannot bind map-binding (key=%q) to nil instance", key))
	}

	// FAIL FAST: Validate assignability
	if !instanceType.AssignableTo(bindtype) && !instanceType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"map-binding validation failed (key=%q): instance of type %s#%s is not assignable to %s#%s",
			key,
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

	// Initialize map if needed
	bindingMap := injector.mapbindings[bindtype]
	if bindingMap == nil {
		bindingMap = make(map[string]*Binding)
	}

	// Store the binding with the key
	bindingMap[key] = binding
	injector.mapbindings[bindtype] = bindingMap

	return binding
}

// BindMapProvider creates a map-binding to a simple provider function with type safety.
// The provider takes no arguments and returns T.
//
// For providers that need dependencies injected, use BindMapProviderFunc.
//
// Example:
//
//	BindMapProvider[Connection](injector, "pool1", func() Connection {
//	    return createConnection("pool1")
//	})
func BindMapProvider[T any](injector *Injector, key string, provider func() T, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create map-binding on nil injector")
	}

	// FAIL FAST: Validate key is not empty
	if key == "" {
		panic("map-binding validation failed: key cannot be empty string")
	}

	if provider == nil {
		panic(fmt.Sprintf("cannot bind map-binding (key=%q) to nil provider", key))
	}

	return BindMapProviderFunc[T](injector, key, provider, opts...)
}

// BindMapProviderWithError creates a map-binding to a provider that can return errors.
//
// Example:
//
//	BindMapProviderWithError[Client](injector, "api", func() (Client, error) {
//	    return createAPIClient()
//	})
func BindMapProviderWithError[T any](injector *Injector, key string, provider func() (T, error), opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create map-binding on nil injector")
	}

	// FAIL FAST: Validate key is not empty
	if key == "" {
		panic("map-binding validation failed: key cannot be empty string")
	}

	if provider == nil {
		panic(fmt.Sprintf("cannot bind map-binding (key=%q) to nil provider", key))
	}

	return BindMapProviderFunc[T](injector, key, provider, opts...)
}

// BindMapProviderFunc creates a map-binding to a provider function with automatic dependency injection.
// The provider can have any parameters, which will be automatically resolved by the injector.
//
// FAIL FAST: Comprehensive validation at binding time.
//
// Example:
//
//	BindMapProviderFunc[Repository](injector, "users", func(db Database) Repository {
//	    return &UserRepository{db: db}
//	})
//
//	BindMapProviderFunc[Cache](injector, "session", func(redis Redis) Cache {
//	    return &SessionCache{redis: redis}
//	}, WithAnnotation("production"), AsSingleton())
func BindMapProviderFunc[T any](injector *Injector, key string, providerFunc interface{}, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create map-binding on nil injector")
	}

	// FAIL FAST: Validate key is not empty
	if key == "" {
		panic("map-binding validation failed: key cannot be empty string")
	}

	if providerFunc == nil {
		panic(fmt.Sprintf("cannot bind map-binding (key=%q) to nil provider", key))
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	if bindtype == nil {
		panic("cannot create map-binding for nil type")
	}

	fnValue := reflect.ValueOf(providerFunc)
	fnType := fnValue.Type()

	// FAIL FAST: Validate provider is a function
	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf(
			"map-binding validation failed (key=%q): provider must be a function, got %s",
			key,
			fnType.Kind(),
		))
	}

	// FAIL FAST: Validate provider has at least one return value
	if fnType.NumOut() == 0 {
		panic(fmt.Sprintf(
			"map-binding validation failed (key=%q): provider function must return at least one value",
			key,
		))
	}

	// FAIL FAST: Validate provider has at most two return values
	if fnType.NumOut() > 2 {
		panic(fmt.Sprintf(
			"map-binding validation failed (key=%q): provider function must return at most 2 values (T or (T, error)), got %d",
			key,
			fnType.NumOut(),
		))
	}

	returnType := fnType.Out(0)

	// FAIL FAST: Validate second return value is error type if present
	if fnType.NumOut() == 2 {
		errorInterface := reflect.TypeOf((*error)(nil)).Elem()
		if !fnType.Out(1).AssignableTo(errorInterface) {
			panic(fmt.Sprintf(
				"map-binding validation failed (key=%q): second return value must be error, got %s",
				key,
				fnType.Out(1),
			))
		}
	}

	// FAIL FAST: Validate return type is assignable to bound type
	if !returnType.AssignableTo(bindtype) && !returnType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"map-binding validation failed (key=%q): provider returns %q which is not assignable to %q",
			key,
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

	// Initialize map if needed
	bindingMap := injector.mapbindings[bindtype]
	if bindingMap == nil {
		bindingMap = make(map[string]*Binding)
	}

	// Store the binding with the key
	bindingMap[key] = binding
	injector.mapbindings[bindtype] = bindingMap

	return binding
}

// ============================================================================
// Injector Helper Functions
// ============================================================================

// GetInstance retrieves an instance of type T from the injector with full type safety.
// This is the generic equivalent of injector.GetInstance() but provides compile-time
// type safety and eliminates the need for type assertions.
//
// Example:
//
//	// Old API:
//	serviceInterface, err := injector.GetInstance((*MyService)(nil))
//	service := serviceInterface.(MyService)
//
//	// New generic API:
//	service, err := GetInstance[MyService](injector)
func GetInstance[T any](injector *Injector) (T, error) {
	var zero T

	if injector == nil {
		return zero, fmt.Errorf("cannot get instance from nil injector")
	}

	// Get the type to resolve
	targetType := reflect.TypeOf((*T)(nil)).Elem()

	// Use the injector's getInstance method
	value, err := injector.getInstance(targetType, "", traceCircular)
	if err != nil {
		return zero, err
	}

	// Convert reflect.Value to T
	result, ok := value.Interface().(T)
	if !ok {
		return zero, fmt.Errorf(
			"type assertion failed: expected %T, got %s",
			zero,
			value.Type(),
		)
	}

	return result, nil
}

// GetAnnotatedInstance retrieves an annotated instance of type T from the injector
// with full type safety.
//
// Example:
//
//	// Retrieve a specific logger implementation
//	fileLogger, err := GetAnnotatedInstance[Logger](injector, "file")
//	consoleLogger, err := GetAnnotatedInstance[Logger](injector, "console")
func GetAnnotatedInstance[T any](injector *Injector, annotation string) (T, error) {
	var zero T

	if injector == nil {
		return zero, fmt.Errorf("cannot get instance from nil injector")
	}

	if annotation == "" {
		return zero, fmt.Errorf("annotation cannot be empty; use GetInstance for non-annotated types")
	}

	// Get the type to resolve
	targetType := reflect.TypeOf((*T)(nil)).Elem()

	// Use the injector's getInstance method with annotation
	value, err := injector.getInstance(targetType, annotation, traceCircular)
	if err != nil {
		return zero, err
	}

	// Convert reflect.Value to T
	result, ok := value.Interface().(T)
	if !ok {
		return zero, fmt.Errorf(
			"type assertion failed: expected %T, got %s",
			zero,
			value.Type(),
		)
	}

	return result, nil
}

// MustGetInstance retrieves an instance of type T from the injector.
// It panics if an error occurs.
//
// This is useful for initialization code where errors should be fatal.
//
// Example:
//
//	config := MustGetInstance[Config](injector)
func MustGetInstance[T any](injector *Injector) T {
	instance, err := GetInstance[T](injector)
	if err != nil {
		panic(fmt.Sprintf("MustGetInstance failed: %v", err))
	}
	return instance
}

// MustGetAnnotatedInstance retrieves an annotated instance of type T from the injector.
// It panics if an error occurs.
//
// Example:
//
//	primaryDB := MustGetAnnotatedInstance[Database](injector, "primary")
func MustGetAnnotatedInstance[T any](injector *Injector, annotation string) T {
	instance, err := GetAnnotatedInstance[T](injector, annotation)
	if err != nil {
		panic(fmt.Sprintf("MustGetAnnotatedInstance failed: %v", err))
	}
	return instance
}

// RequestInjection requests injection for the given object with type safety.
// This is the generic equivalent of injector.RequestInjection() but provides
// better type information at compile time.
//
// Example:
//
//	service := &MyService{}
//	err := RequestInjection(injector, service)
func RequestInjection[T any](injector *Injector, object T) error {
	if injector == nil {
		return fmt.Errorf("cannot request injection on nil injector")
	}

	// Validate object is not nil
	objValue := reflect.ValueOf(object)
	if !objValue.IsValid() || (objValue.Kind() == reflect.Ptr && objValue.IsNil()) {
		return fmt.Errorf("cannot request injection on nil object")
	}

	return injector.requestInjection(object, traceCircular)
}

// MustRequestInjection requests injection for the given object.
// It panics if an error occurs.
//
// Example:
//
//	service := &MyService{}
//	MustRequestInjection(injector, service)
func MustRequestInjection[T any](injector *Injector, object T) {
	if err := RequestInjection(injector, object); err != nil {
		panic(fmt.Sprintf("MustRequestInjection failed: %v", err))
	}
}

// Override creates a typed override for an existing binding from type F to type T.
// This is useful for testing or providing alternative implementations.
//
// The annotation parameter specifies which binding to override (empty string for non-annotated bindings).
//
// Example:
//
//	// In tests, override production database with mock
//	Override[Database, *MockDB](injector, "")
//
//	// Override annotated binding
//	Override[Logger, *TestLogger](injector, "file", AsSingleton())
func Override[F, T any](injector *Injector, annotation string, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create override on nil injector")
	}

	fromType := reflect.TypeOf((*F)(nil)).Elem()
	toType := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate types
	if fromType == nil {
		panic("cannot create override from nil type")
	}
	if toType == nil {
		panic("cannot create override to nil type")
	}

	// Handle pointer types
	actualToType := toType
	for actualToType.Kind() == reflect.Ptr {
		actualToType = actualToType.Elem()
	}

	// FAIL FAST: Validate assignability at binding time
	if !actualToType.AssignableTo(fromType) && !reflect.PtrTo(actualToType).AssignableTo(fromType) {
		panic(fmt.Sprintf(
			"override validation failed: %s#%s is not assignable to %s#%s",
			actualToType.PkgPath(), actualToType.Name(),
			fromType.PkgPath(), fromType.Name(),
		))
	}

	binding := &Binding{
		typeof:        fromType,
		to:            actualToType,
		annotatedWith: annotation,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
	}

	// Add to overrides list
	injector.overrides = append(injector.overrides, &override{
		typ:           fromType,
		annotatedWith: annotation,
		binding:       binding,
	})

	return binding
}

// OverrideInstance creates a typed override for an existing binding using an instance.
// This is useful for testing or providing pre-configured instances.
//
// Example:
//
//	mockDB := &MockDB{}
//	OverrideInstance[Database](injector, "", mockDB)
func OverrideInstance[T any](injector *Injector, annotation string, instance T, opts ...BindingOption) *Binding {
	if injector == nil {
		panic("cannot create override on nil injector")
	}

	bindtype := reflect.TypeOf((*T)(nil)).Elem()
	instanceType := reflect.TypeOf(instance)
	instanceValue := reflect.ValueOf(instance)

	// FAIL FAST: Validate types
	if bindtype == nil {
		panic("cannot create override for nil type")
	}
	if instanceType == nil {
		panic("cannot create override with nil instance")
	}

	// FAIL FAST: Validate assignability
	if !instanceType.AssignableTo(bindtype) && !instanceType.AssignableTo(reflect.PtrTo(bindtype)) {
		panic(fmt.Sprintf(
			"override validation failed: instance of type %s#%s is not assignable to %s#%s",
			instanceType.PkgPath(), instanceType.Name(),
			bindtype.PkgPath(), bindtype.Name(),
		))
	}

	binding := &Binding{
		typeof:        bindtype,
		annotatedWith: annotation,
		instance: &Instance{
			itype:  instanceType,
			ivalue: instanceValue,
		},
	}

	// Apply functional options
	for _, opt := range opts {
		opt(binding)
	}

	// Add to overrides list
	injector.overrides = append(injector.overrides, &override{
		typ:           bindtype,
		annotatedWith: annotation,
		binding:       binding,
	})

	return binding
}

// BindInterceptor binds an interceptor for type T.
// This provides a type-safe way to set up aspect-oriented programming (AOP) interceptors.
//
// FAIL FAST: Validation ensures T is an interface type.
//
// Example:
//
//	type LoggingInterceptor struct {
//	    Target MyInterface
//	}
//
//	BindInterceptor[MyInterface](injector, LoggingInterceptor{})
func BindInterceptor[T any](injector *Injector, interceptor interface{}) {
	if injector == nil {
		panic("cannot bind interceptor on nil injector")
	}

	totype := reflect.TypeOf((*T)(nil)).Elem()

	// FAIL FAST: Validate T is an interface
	if totype.Kind() != reflect.Interface {
		panic(fmt.Sprintf(
			"interceptor validation failed: can only intercept interfaces, got %s",
			totype.Kind(),
		))
	}

	interceptorType := reflect.TypeOf(interceptor)
	if interceptorType == nil {
		panic("cannot bind nil interceptor")
	}

	// For pointer types, get the element
	for interceptorType.Kind() == reflect.Ptr {
		interceptorType = interceptorType.Elem()
	}

	// FAIL FAST: Validate interceptor is a struct
	if interceptorType.Kind() != reflect.Struct {
		panic(fmt.Sprintf(
			"interceptor validation failed: interceptor must be a struct, got %s",
			interceptorType.Kind(),
		))
	}

	// FAIL FAST: Validate interceptor has at least one field
	if interceptorType.NumField() == 0 {
		panic("interceptor validation failed: interceptor struct must have at least one field")
	}

	// FAIL FAST: Validate first field is assignable from T
	firstField := interceptorType.Field(0)
	if !totype.AssignableTo(firstField.Type) {
		panic(fmt.Sprintf(
			"interceptor validation failed: first field of interceptor must be of type %s, got %s",
			totype,
			firstField.Type,
		))
	}

	// Add to interceptor map
	m := injector.interceptor[totype]
	m = append(m, interceptorType)
	injector.interceptor[totype] = m
}

// GetMultiInstance retrieves all instances bound via BindMulti as a slice.
// This is a convenience method that's equivalent to GetInstance[[]T].
//
// Example:
//
//	plugins, err := GetMultiInstance[Plugin](injector)
//	for _, plugin := range plugins {
//	    plugin.Initialize()
//	}
func GetMultiInstance[T any](injector *Injector) ([]T, error) {
	return GetInstance[[]T](injector)
}

// GetMultiAnnotatedInstance retrieves all annotated instances bound via BindMulti.
//
// Example:
//
//	prodPlugins, err := GetMultiAnnotatedInstance[Plugin](injector, "production")
func GetMultiAnnotatedInstance[T any](injector *Injector, annotation string) ([]T, error) {
	return GetAnnotatedInstance[[]T](injector, annotation)
}

// GetMapInstance retrieves all instances bound via BindMap as a map.
// This is a convenience method that's equivalent to GetInstance[map[string]T].
//
// Example:
//
//	databases, err := GetMapInstance[Database](injector)
//	primaryDB := databases["primary"]
//	replicaDB := databases["replica"]
func GetMapInstance[T any](injector *Injector) (map[string]T, error) {
	return GetInstance[map[string]T](injector)
}

// GetMapAnnotatedInstance retrieves all annotated instances bound via BindMap.
//
// Example:
//
//	prodDBs, err := GetMapAnnotatedInstance[Database](injector, "production")
func GetMapAnnotatedInstance[T any](injector *Injector, annotation string) (map[string]T, error) {
	return GetAnnotatedInstance[map[string]T](injector, annotation)
}

// GetMapKey retrieves a single instance from a map binding by key.
// This is a convenience method that's equivalent to using the "map:key" annotation.
//
// Example:
//
//	// Instead of:
//	db, err := GetAnnotatedInstance[Database](injector, "map:primary")
//
//	// You can use:
//	db, err := GetMapKey[Database](injector, "primary")
func GetMapKey[T any](injector *Injector, key string) (T, error) {
	if key == "" {
		var zero T
		return zero, fmt.Errorf("map key cannot be empty")
	}
	return GetAnnotatedInstance[T](injector, "map:"+key)
}

// MustGetMapKey retrieves a single instance from a map binding by key.
// It panics if an error occurs.
//
// Example:
//
//	primaryDB := MustGetMapKey[Database](injector, "primary")
func MustGetMapKey[T any](injector *Injector, key string) T {
	instance, err := GetMapKey[T](injector, key)
	if err != nil {
		panic(fmt.Sprintf("MustGetMapKey failed: %v", err))
	}
	return instance
}

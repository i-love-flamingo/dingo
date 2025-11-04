package dingo

import (
	"fmt"
	"reflect"
)

// Helper functions for generic binding operations
//
// These functions provide common operations for type manipulation and validation
// used across all binding functions (Bind, BindLike, BindInstance, BindProvider, BindMulti).

// stripPtrType removes one level of pointer indirection from a type.
//
// If the type is not a pointer, it returns the type unchanged. This is used
// to normalize type parameters that may or may not be specified with a pointer.
//
// Example:
//
//	stripPtrType(reflect.TypeOf((*string)(nil))) → string
//	stripPtrType(reflect.TypeOf("hello"))        → string
func stripPtrType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		return t.Elem()
	}

	return t
}

// stripAllPtrs removes all levels of pointer indirection from a type.
//
// It keeps unwrapping pointers until it reaches a non-pointer type. This is
// useful for normalizing types like **T or ***T to T.
//
// Example:
//
//	stripAllPtrs(reflect.TypeOf((***int)(nil))) → int
func stripAllPtrs(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t
}

// typeForNoPtr returns the reflect.Type for T with any pointer level stripped.
//
// This is a convenience wrapper around reflect.TypeFor and stripPtrType that
// combines getting the type of a generic parameter and normalizing it to a
// non-pointer type in one operation.
//
// Example:
//
//	typeForNoPtr[*UserService]() → UserService type
//	typeForNoPtr[UserService]()  → UserService type
func typeForNoPtr[T any]() reflect.Type {
	return stripPtrType(reflect.TypeFor[T]())
}

// isAssignable checks if the 'from' type can be assigned to the 'to' type.
//
// It considers both direct assignment (from → to) and pointer assignment (*from → to).
// This is necessary because both value types and pointer types can implement interfaces.
//
// Example:
//
//	isAssignable(ConsoleLogger, Logger)  → true if ConsoleLogger implements Logger
//	isAssignable(ConsoleLogger, Logger)  → true if *ConsoleLogger implements Logger
func isAssignable(from, to reflect.Type) bool {
	return from.AssignableTo(to) || reflect.PointerTo(from).AssignableTo(to)
}

// formatTypeName formats a reflect.Type for display in error messages.
//
// It handles builtin types (which have no PkgPath) and fully-qualified types
// with their package paths, providing clear and consistent type names in errors.
//
// Example:
//
//	formatTypeName(string type)      → "string"
//	formatTypeName(UserService type) → "github.com/user/pkg.UserService"
func formatTypeName(t reflect.Type) string {
	if t.PkgPath() == "" {
		// Builtin type (string, int, etc.) or unnamed type
		return t.String()
	}

	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

// formatTypeNotAssignableError creates a consistent error message for type mismatch errors.
//
// It includes the function context (e.g., "Bind[T, U]") and properly formatted
// type names to make debugging type errors easier.
//
// Example error:
//
//	"dingo: Bind[T, U]: type int is not assignable to github.com/user/pkg.Logger"
func formatTypeNotAssignableError(from, to reflect.Type, context string) string {
	return fmt.Sprintf("dingo: %s: type %s is not assignable to %s",
		context,
		formatTypeName(from),
		formatTypeName(to))
}

// Bind creates a generic type binding from interface T to implementation U.
//
// Bind automatically strips pointer types from both T and U, and validates
// that U is assignable to T at binding time. If types are incompatible,
// it panics with a descriptive error.
//
// Type Parameters:
//   - T: The type to bind (typically an interface or abstract type)
//   - U: The concrete implementation type
//
// Parameters:
//   - injector: The injector to add the binding to
//
// Returns:
//   - *Binding: A chainable binding that can be configured with AnnotatedWith(), In(), etc.
//
// Example - Simple binding:
//
//	type Logger interface {
//	    Log(msg string)
//	}
//	type ConsoleLogger struct{}
//	func (c *ConsoleLogger) Log(msg string) { fmt.Println(msg) }
//
//	Bind[Logger, ConsoleLogger](injector)
//
// Example - With annotation and scope:
//
//	Bind[Logger, FileLogger](injector).
//	    AnnotatedWith("file").
//	    In(Singleton)
//
// Example - Interface to implementation:
//
//	type UserService interface {
//	    GetUser(id string) (*User, error)
//	}
//	type UserServiceImpl struct {
//	    db *Database `inject:""`
//	}
//
//	Bind[UserService, UserServiceImpl](injector)
//
// Panics if:
//   - injector is nil
//   - U is not assignable to T (checked at binding time)
func Bind[T, U any](injector *Injector) *Binding {
	if injector == nil {
		panic("dingo: Bind[T, U]: injector cannot be nil")
	}

	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := stripAllPtrs(reflect.TypeFor[U]())

	if !isAssignable(to, binding.typeof) {
		panic(formatTypeNotAssignableError(to, binding.typeof, "Bind[T, U]"))
	}

	binding.to = to

	return binding
}

// BindLike binds type T to the same concrete type as the provided example value.
//
// IMPORTANT: The example value is ONLY used to determine the concrete type to bind to.
// The value itself is NOT stored or used as an instance. If you want to bind a specific
// instance, use BindInstance instead.
//
// This is useful when you want to avoid repeating the full type name, especially for
// complex types, and let Go infer it from a value.
//
// Type Parameters:
//   - T: The type to bind (typically an interface)
//
// Parameters:
//   - injector: The injector to add the binding to
//   - example: An example value whose type will be used for the binding
//
// Returns:
//   - *Binding: A chainable binding that can be configured with AnnotatedWith(), In(), etc.
//
// Example - Bind interface to implementation type:
//
//	type UserService interface {
//	    GetUser(id int) (*User, error)
//	}
//	type UserServiceImpl struct {
//	    db *Database
//	}
//
//	impl := &UserServiceImpl{}
//	BindLike[UserService](injector, impl)  // Binds UserService to *UserServiceImpl type
//
// This creates NEW instances of *UserServiceImpl when UserService is requested,
// it does NOT reuse the 'impl' variable. Compare with BindInstance:
//
//	BindInstance[UserService](injector, impl)  // Binds the actual instance
//
// For singleton behavior with BindLike:
//
//	BindLike[UserService](injector, impl).In(Singleton)
//
// Panics if:
//   - injector is nil
//   - The example's type is not assignable to T
func BindLike[T any](injector *Injector, example T) *Binding {
	if injector == nil {
		panic("dingo: BindLike[T]: injector cannot be nil")
	}

	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	to := stripAllPtrs(reflect.TypeOf(example))

	if !isAssignable(to, binding.typeof) {
		panic(formatTypeNotAssignableError(to, binding.typeof, "BindLike[T]"))
	}

	binding.to = to

	return binding
}

// BindInstance binds type T to a specific instance value.
//
// The provided instance is stored and reused for all injection requests.
// This is similar to singleton behavior, but the instance is provided at
// binding time rather than created lazily.
//
// Type Parameters:
//   - T: The type to bind (can be interface, struct, or primitive)
//
// Parameters:
//   - injector: The injector to add the binding to
//   - instance: The specific instance to bind and reuse
//
// Returns:
//   - *Binding: A chainable binding that can be configured with AnnotatedWith()
//
// Example - Bind configuration value:
//
//	config := &AppConfig{Port: 8080, Debug: true}
//	BindInstance[*AppConfig](injector, config)
//
// Example - Bind interface to specific instance:
//
//	logger := &ConsoleLogger{Level: "INFO"}
//	BindInstance[Logger](injector, logger)
//
// Example - With annotation:
//
//	BindInstance[string](injector, "production").AnnotatedWith("config:env")
//
// Use cases:
//   - Configuration values (ports, URLs, feature flags)
//   - Pre-initialized connections (database, HTTP clients)
//   - Test doubles and mocks
//
// Note: Unlike BindLike, this binds the ACTUAL instance, not just the type.
//
// Panics if:
//   - injector is nil
//   - The instance type is not assignable to T
func BindInstance[T any](injector *Injector, instance T) *Binding {
	if injector == nil {
		panic("dingo: BindInstance[T]: injector cannot be nil")
	}

	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	binding.instance = &Instance{
		itype:  reflect.TypeOf(instance),
		ivalue: reflect.ValueOf(instance),
	}
	// For BindInstance, we check if itype is assignable to typeof or to *typeof
	// (not if *itype is assignable to typeof, which is what isAssignable does)
	if !binding.instance.itype.AssignableTo(binding.typeof) && !binding.instance.itype.AssignableTo(reflect.PointerTo(binding.typeof)) {
		panic(formatTypeNotAssignableError(binding.instance.itype, binding.typeof, "BindInstance[T]"))
	}

	return binding
}

// BindProvider binds type T to a provider function that creates instances.
//
// The provider function can have dependencies as parameters, which will be
// automatically injected by dingo when the provider is called. The provider
// is called each time an instance of T is requested (unless scoped).
//
// Type Parameters:
//   - T: The type to bind (return type of the provider function)
//
// Parameters:
//   - injector: The injector to add the binding to
//   - providerFunc: A function that returns T (can have any number of injected parameters)
//
// Returns:
//   - *Binding: A chainable binding that can be configured with AnnotatedWith(), In(), etc.
//
// Example - Provider with no dependencies:
//
//	BindProvider[Logger](injector, func() Logger {
//	    return &ConsoleLogger{Level: "INFO"}
//	})
//
// Example - Provider with injected dependencies:
//
//	BindProvider[UserService](injector, func(db *Database, logger Logger) UserService {
//	    return &UserServiceImpl{
//	        db:     db,
//	        logger: logger,
//	    }
//	})
//
// Example - Provider with singleton scope:
//
//	BindProvider[Database](injector, func(config *DBConfig) Database {
//	    db, _ := sql.Open("postgres", config.ConnectionString)
//	    return db
//	}).In(Singleton)
//
// Use cases:
//   - Complex initialization logic
//   - Conditional construction based on dependencies
//   - Integration with external factories
//   - Lazy initialization
//
// Note: For type-safe variants with compile-time checking, see BindProvider0,
// BindProvider1, BindProvider2, and BindProvider3.
//
// Panics if:
//   - injector is nil
//   - providerFunc is not a function
//   - providerFunc does not return a value
//   - providerFunc's return type is not assignable to T
func BindProvider[T any](injector *Injector, providerFunc any) *Binding {
	if injector == nil {
		panic("dingo: BindProvider[T]: injector cannot be nil")
	}

	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype
	injector.bindings[bindtype] = append(injector.bindings[bindtype], binding)

	// Validate provider function early
	fnValue := reflect.ValueOf(providerFunc)
	if fnValue.Kind() != reflect.Func {
		panic(fmt.Sprintf("dingo: BindProvider[%s]: expected function, got %s",
			formatTypeName(bindtype), fnValue.Kind()))
	}

	fnType := fnValue.Type()
	if fnType.NumOut() == 0 {
		panic(fmt.Sprintf("dingo: BindProvider[%s]: provider function must return at least one value",
			formatTypeName(bindtype)))
	}

	provider := &Provider{
		fnc:     fnValue,
		binding: binding,
	}

	provider.fnctype = fnType.Out(0)
	// For BindProvider, we check if fnctype is assignable to typeof or to *typeof
	// (not if *fnctype is assignable to typeof, which is what isAssignable does)
	if !provider.fnctype.AssignableTo(binding.typeof) && !provider.fnctype.AssignableTo(reflect.PointerTo(binding.typeof)) {
		panic(formatTypeNotAssignableError(provider.fnctype, binding.typeof, "BindProvider[T]"))
	}

	binding.provider = provider

	return binding
}

// BindMulti creates a multibinding from interface T to implementation U.
//
// Multibindings allow multiple implementations to be bound to the same interface.
// When injected, they appear as a slice []T containing all bound implementations.
// This enables plugin patterns and registries.
//
// Type Parameters:
//   - T: The type to bind (typically an interface)
//   - U: The concrete implementation type to add to the multibinding
//
// Parameters:
//   - injector: The injector to add the binding to
//
// Returns:
//   - *Binding: A chainable binding that can be configured with AnnotatedWith(), In(), etc.
//
// Example - Multiple handlers:
//
//	type Handler interface {
//	    Handle(req *Request) error
//	}
//	type LoggingHandler struct{}
//	type AuthHandler struct{}
//	type ValidationHandler struct{}
//
//	BindMulti[Handler, LoggingHandler](injector)
//	BindMulti[Handler, AuthHandler](injector)
//	BindMulti[Handler, ValidationHandler](injector)
//
//	type App struct {
//	    Handlers []Handler `inject:""`  // Contains all three handlers
//	}
//
// Example - Plugin system with annotations:
//
//	BindMulti[Plugin, DatabasePlugin](injector).AnnotatedWith("plugins")
//	BindMulti[Plugin, CachePlugin](injector).AnnotatedWith("plugins")
//
//	type System struct {
//	    Plugins []Plugin `inject:"plugins"`
//	}
//
// Use cases:
//   - Middleware pipelines
//   - Event handlers
//   - Plugin systems
//   - Strategy pattern with multiple strategies
//
// Panics if:
//   - injector is nil
//   - U is not assignable to T
func BindMulti[T, U any](injector *Injector) *Binding {
	if injector == nil {
		panic("dingo: BindMulti[T, U]: injector cannot be nil")
	}

	bindtype := typeForNoPtr[T]()

	binding := new(Binding)
	binding.typeof = bindtype

	to := stripAllPtrs(reflect.TypeFor[U]())

	if !isAssignable(to, binding.typeof) {
		panic(formatTypeNotAssignableError(to, binding.typeof, "BindMulti[T, U]"))
	}

	binding.to = to

	injector.multibindings[bindtype] = append(injector.multibindings[bindtype], binding)

	return binding
}

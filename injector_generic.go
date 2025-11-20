package dingo

import (
	"fmt"
	"reflect"
)

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

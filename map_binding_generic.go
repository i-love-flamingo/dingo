package dingo

import (
	"fmt"
	"reflect"
)

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

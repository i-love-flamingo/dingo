package dingo

// This file contains examples demonstrating the new generic binding API.
// The new API provides type safety, ergonomic usage, and fail-fast validation
// while remaining fully compatible with the existing API.
//
// Key Features:
// - Type-safe bindings using Go generics
// - Fail-fast validation at binding time (not resolution time)
// - Clean, idiomatic Go API design
// - Full compatibility with existing non-generic API
// - Returns *Binding for method chaining with existing methods

// Example interfaces and types used in examples below

type Logger interface {
	Log(message string)
}

type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(message string) {}

type FileLogger struct {
	filename string
}

func (f *FileLogger) Log(message string) {}

type Database interface {
	Query(sql string) error
}

type PostgresDB struct {
	connectionString string
}

func (p *PostgresDB) Query(sql string) error { return nil }

type MySQLDB struct {
	connectionString string
}

func (m *MySQLDB) Query(sql string) error { return nil }

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
}

type RedisCache struct{}

func (r *RedisCache) Get(key string) (interface{}, error) { return nil, nil }
func (r *RedisCache) Set(key string, value interface{}) error { return nil }

type MemoryCache struct{}

func (m *MemoryCache) Get(key string) (interface{}, error) { return nil, nil }
func (m *MemoryCache) Set(key string, value interface{}) error { return nil }

type Plugin interface {
	Initialize() error
	Execute() error
}

type PluginA struct{}

func (p *PluginA) Initialize() error { return nil }
func (p *PluginA) Execute() error    { return nil }

type PluginB struct{}

func (p *PluginB) Initialize() error { return nil }
func (p *PluginB) Execute() error    { return nil }

// Example 1: Basic Binding
// Shows the simplest usage of the generic API
func ExampleBasicBinding() {
	injector, _ := NewInjector()

	// Old API:
	// injector.Bind((*Logger)(nil)).To(ConsoleLogger{})

	// New generic API - type-safe and clean:
	Bind[Logger](injector).To(&ConsoleLogger{})

	// Retrieve instance with type safety (no type assertion needed!)
	logger, _ := GetInstance[Logger](injector)
	logger.Log("Hello, world!")
}

// Example 2: Instance Binding
// Shows how to bind to pre-configured instances
func ExampleInstanceBinding() {
	injector, _ := NewInjector()

	// Create and configure an instance
	fileLogger := &FileLogger{filename: "/var/log/app.log"}

	// Old API:
	// injector.Bind((*Logger)(nil)).ToInstance(fileLogger)

	// New generic API with convenience function:
	BindInstance[Logger](injector, fileLogger)

	// Or use the standard Bind + ToInstance:
	Bind[Logger](injector).ToInstance(fileLogger)

	// Retrieve the same instance
	logger, _ := GetInstance[Logger](injector)
	logger.Log("Logging to file")
}

// Example 3: Type-Safe Binding with BindTo
// Shows explicit type relationships for better validation
func ExampleBindTo() {
	injector, _ := NewInjector()

	// BindTo validates at binding time that PostgresDB implements Database
	BindTo[Database, *PostgresDB](injector).In(Singleton)

	// This would fail at binding time (compile error or panic):
	// BindTo[Database, string](injector) // PANIC: string doesn't implement Database

	db, _ := GetInstance[Database](injector)
	db.Query("SELECT * FROM users")
}

// Example 4: Provider Binding
// Shows how to use factory functions
func ExampleProviderBinding() {
	injector, _ := NewInjector()

	// Simple provider (no dependencies)
	BindProvider[Logger](injector, func() Logger {
		return &ConsoleLogger{}
	})

	// Provider with dependencies (auto-injected)
	BindInstance[Database](injector, &PostgresDB{connectionString: "localhost"})

	// This provider receives the Database dependency automatically
	BindProviderFunc[Cache](injector, func(db Database) Cache {
		return &RedisCache{} // db is automatically injected
	})

	cache, _ := GetInstance[Cache](injector)
	cache.Set("key", "value")
}

// Example 5: Provider with Error Handling
// Shows providers that can fail gracefully
func ExampleProviderWithError() {
	injector, _ := NewInjector()

	BindProviderWithError[Database](injector, func() (Database, error) {
		// Simulate connection
		db := &PostgresDB{connectionString: "localhost"}
		// Could return error here if connection fails
		return db, nil
	})

	db, err := GetInstance[Database](injector)
	if err != nil {
		panic(err)
	}
	db.Query("SELECT * FROM users")
}

// Example 6: Annotated Bindings
// Shows multiple bindings of the same type with annotations
func ExampleAnnotatedBindings() {
	injector, _ := NewInjector()

	// Bind multiple implementations with different annotations
	Bind[Logger](injector).
		To(&ConsoleLogger{}).
		AnnotatedWith("console")

	Bind[Logger](injector).
		To(&FileLogger{filename: "/var/log/app.log"}).
		AnnotatedWith("file")

	// Retrieve specific implementations
	consoleLogger, _ := GetAnnotatedInstance[Logger](injector, "console")
	fileLogger, _ := GetAnnotatedInstance[Logger](injector, "file")

	consoleLogger.Log("Console message")
	fileLogger.Log("File message")
}

// Example 7: Scoped Bindings
// Shows Singleton and other scope usage
func ExampleScopedBindings() {
	injector, _ := NewInjector()

	// Singleton - same instance across entire app
	Bind[Database](injector).
		To(&PostgresDB{connectionString: "localhost"}).
		In(Singleton)

	// Or use convenience method:
	BindTo[Database, *PostgresDB](injector).AsEagerSingleton()

	// Child Singleton - new instance per child injector
	Bind[Cache](injector).To(&RedisCache{}).In(ChildSingleton)

	db1, _ := GetInstance[Database](injector)
	db2, _ := GetInstance[Database](injector)
	// db1 and db2 are the same instance (Singleton)
	_ = db1
	_ = db2
}

// Example 8: Multi-Bindings
// Shows how to register multiple implementations as a slice
func ExampleMultiBindings() {
	injector, _ := NewInjector()

	// Register multiple plugins
	BindMulti[Plugin](injector).To(&PluginA{})
	BindMulti[Plugin](injector).To(&PluginB{})

	// Or use convenience functions:
	BindMultiInstance[Plugin](injector, &PluginA{})

	// Retrieve all plugins as a slice (type-safe!)
	plugins, _ := GetMultiInstance[Plugin](injector)

	for _, plugin := range plugins {
		plugin.Initialize()
	}
}

// Example 9: Annotated Multi-Bindings
// Shows grouping multi-bindings by annotation
func ExampleAnnotatedMultiBindings() {
	injector, _ := NewInjector()

	// Production plugins
	BindMulti[Plugin](injector).
		To(&PluginA{}).
		AnnotatedWith("production")

	BindMulti[Plugin](injector).
		To(&PluginB{}).
		AnnotatedWith("production")

	// Test plugins
	BindMulti[Plugin](injector).
		To(&PluginA{}).
		AnnotatedWith("test")

	// Retrieve only production plugins
	prodPlugins, _ := GetMultiAnnotatedInstance[Plugin](injector, "production")

	for _, plugin := range prodPlugins {
		plugin.Execute()
	}
}

// Example 10: Map Bindings
// Shows registry-style key-value bindings
func ExampleMapBindings() {
	injector, _ := NewInjector()

	// Register multiple databases with keys
	BindMap[Database](injector, "primary").
		To(&PostgresDB{connectionString: "primary.db"})

	BindMap[Database](injector, "replica").
		To(&PostgresDB{connectionString: "replica.db"})

	// Or use convenience functions:
	BindMapInstance[Database](injector, "analytics",
		&MySQLDB{connectionString: "analytics.db"})

	// Retrieve all as a map
	databases, _ := GetMapInstance[Database](injector)
	primaryDB := databases["primary"]
	primaryDB.Query("INSERT INTO users ...")

	// Or retrieve individual database by key
	analyticsDB, _ := GetMapKey[Database](injector, "analytics")
	analyticsDB.Query("SELECT COUNT(*) ...")
}

// Example 11: Struct Field Injection
// Shows how struct tags work with the generic API
func ExampleStructFieldInjection() {
	injector, _ := NewInjector()

	// Set up bindings using generic API
	Bind[Logger](injector).To(&ConsoleLogger{})
	BindTo[Database, *PostgresDB](injector)
	BindInstance[Cache](injector, &RedisCache{})

	// Define a service with injected dependencies
	type UserService struct {
		Logger Logger   `inject:""`
		DB     Database `inject:""`
		Cache  Cache    `inject:""`
	}

	// Create and inject
	service := &UserService{}
	RequestInjection(injector, service)

	// Dependencies are now injected
	service.Logger.Log("UserService initialized")
	service.DB.Query("SELECT * FROM users")
}

// Example 12: Override Bindings (for testing)
// Shows how to override bindings for testing
func ExampleOverrideBindings() {
	injector, _ := NewInjector()

	// Original binding
	Bind[Database](injector).To(&PostgresDB{})

	// In tests, override with mock
	type MockDB struct{}

	func (m *MockDB) Query(sql string) error { return nil }

	Override[Database](injector, "").ToInstance(&MockDB{})

	// GetInstance now returns the mock
	db, _ := GetInstance[Database](injector)
	_ = db // This is the MockDB
}

// Example 13: Interceptors (AOP)
// Shows aspect-oriented programming with the generic API
func ExampleInterceptors() {
	injector, _ := NewInjector()

	// Define an interceptor
	type LoggingInterceptor struct {
		Target Database
		Logger Logger
	}

	func (l *LoggingInterceptor) Query(sql string) error {
		l.Logger.Log("Executing query: " + sql)
		return l.Target.Query(sql)
	}

	// Set up bindings
	Bind[Logger](injector).To(&ConsoleLogger{})
	Bind[Database](injector).To(&PostgresDB{})

	// Bind interceptor
	BindInterceptor[Database](injector, LoggingInterceptor{})

	// The returned database will be wrapped with logging
	db, _ := GetInstance[Database](injector)
	db.Query("SELECT * FROM users") // Logs before executing
}

// Example 14: Complex Provider Dependencies
// Shows providers with multiple auto-injected dependencies
func ExampleComplexProviderDependencies() {
	injector, _ := NewInjector()

	Bind[Logger](injector).To(&ConsoleLogger{})
	Bind[Database](injector).To(&PostgresDB{})

	// Provider with multiple dependencies
	BindProviderFunc[Cache](injector, func(logger Logger, db Database) Cache {
		logger.Log("Creating cache with database backing")
		return &RedisCache{}
	})

	cache, _ := GetInstance[Cache](injector)
	cache.Set("key", "value")
}

// Example 15: Full Application Example
// Shows a complete application using the generic API
func ExampleFullApplication() {
	// Create injector with modules
	injector, _ := NewInjector(ModuleFunc(func(i *Injector) {
		// Infrastructure bindings
		Bind[Logger](i).To(&ConsoleLogger{}).In(Singleton)

		BindProviderWithError[Database](i, func() (Database, error) {
			return &PostgresDB{connectionString: "localhost"}, nil
		}).In(Singleton)

		BindInstance[Cache](i, &RedisCache{}).In(Singleton)

		// Service bindings
		type UserRepository struct {
			DB Database `inject:""`
		}

		type UserService struct {
			Repo   *UserRepository `inject:""`
			Logger Logger          `inject:""`
			Cache  Cache           `inject:""`
		}

		// Multi-bindings for plugins
		BindMulti[Plugin](i).To(&PluginA{})
		BindMulti[Plugin](i).To(&PluginB{})
	}))

	// Application startup
	type Application struct {
		Plugins []Plugin `inject:""`
	}

	app := &Application{}
	MustRequestInjection(injector, app)

	// Initialize all plugins
	for _, plugin := range app.Plugins {
		plugin.Initialize()
	}
}

// Example 16: Compatibility with Old API
// Shows that the new API is fully compatible with the old API
func ExampleCompatibilityWithOldAPI() {
	injector, _ := NewInjector()

	// Mix old and new API freely

	// Old API
	injector.Bind((*Logger)(nil)).To(ConsoleLogger{})

	// New API
	Bind[Database](injector).To(&PostgresDB{})

	// Retrieve with old API
	loggerInterface, _ := injector.GetInstance((*Logger)(nil))
	logger := loggerInterface.(Logger)

	// Retrieve with new API (no type assertion!)
	db, _ := GetInstance[Database](injector)

	logger.Log("Mixed API usage")
	db.Query("SELECT 1")
}

// Example 17: Fail-Fast Validation
// Shows how the new API validates at binding time, not resolution time
func ExampleFailFastValidation() {
	injector, _ := NewInjector()

	// All these will PANIC at binding time (not resolution time):

	// Invalid: string doesn't implement Logger
	// Bind[Logger](injector).To("not a logger") // PANIC!

	// Invalid: empty annotation
	// Bind[Logger](injector).AnnotatedWith("") // PANIC!

	// Invalid: nil scope
	// Bind[Logger](injector).In(nil) // PANIC!

	// Invalid: nil provider
	// BindProvider[Logger](injector, nil) // PANIC!

	// Invalid: provider with wrong return type
	// BindProviderFunc[Logger](injector, func() string { return "wrong" }) // PANIC!

	// All validation happens at binding time, catching errors early
	_ = injector
}

// Example 18: API Comparison
// Direct comparison between old and new API
func ExampleAPIComparison() {
	injector, _ := NewInjector()

	// ===== BASIC BINDING =====
	// Old: injector.Bind((*Logger)(nil)).To(ConsoleLogger{})
	// New: Bind[Logger](injector).To(&ConsoleLogger{})

	// ===== INSTANCE BINDING =====
	logger := &ConsoleLogger{}
	// Old: injector.Bind((*Logger)(nil)).ToInstance(logger)
	// New: BindInstance[Logger](injector, logger)

	// ===== PROVIDER BINDING =====
	// Old: injector.Bind((*Logger)(nil)).ToProvider(func() interface{} { return &ConsoleLogger{} })
	// New: BindProvider[Logger](injector, func() Logger { return &ConsoleLogger{} })

	// ===== MULTI-BINDING =====
	// Old: injector.BindMulti((*Plugin)(nil)).To(PluginA{})
	// New: BindMulti[Plugin](injector).To(&PluginA{})

	// ===== MAP BINDING =====
	// Old: injector.BindMap((*Database)(nil), "primary").To(PostgresDB{})
	// New: BindMap[Database](injector, "primary").To(&PostgresDB{})

	// ===== GET INSTANCE =====
	// Old: dbInterface, _ := injector.GetInstance((*Database)(nil)); db := dbInterface.(Database)
	// New: db, _ := GetInstance[Database](injector) // No type assertion!

	// The new API is cleaner, type-safer, and more ergonomic!
	_ = injector
	_ = logger
}

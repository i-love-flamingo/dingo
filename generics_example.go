package dingo

// This file demonstrates the new Go-idiomatic generic binding API.
//
// Key Features:
// - Type-safe bindings using Go generics
// - Functional options pattern (no chaining)
// - Combined Bind+To in single function call
// - Fail-fast validation at binding time
// - Clean, concise API surface
// - Full compatibility with existing API

// Example interfaces and types

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

// ====================
// Example 1: Basic Binding
// ====================
func ExampleBasicBinding() {
	injector, _ := NewInjector()

	// Old API (chaining):
	// injector.Bind((*Logger)(nil)).To(ConsoleLogger{})

	// Previous generic API (chaining):
	// Bind[Logger](injector).To(&ConsoleLogger{})

	// New API (functional options, no chaining):
	Bind[Logger, *ConsoleLogger](injector)

	// Retrieve with type safety (no type assertions!)
	logger, _ := GetInstance[Logger](injector)
	logger.Log("Hello, world!")
}

// ====================
// Example 2: Binding with Options
// ====================
func ExampleBindingWithOptions() {
	injector, _ := NewInjector()

	// Combine binding with annotation and scope using functional options
	Bind[Logger, *ConsoleLogger](injector, WithAnnotation("console"), AsSingleton())
	Bind[Logger, *FileLogger](injector, WithAnnotation("file"), WithScope(ChildSingleton))

	// Clean, concise, and readable!
	consoleLogger, _ := GetAnnotatedInstance[Logger](injector, "console")
	fileLogger, _ := GetAnnotatedInstance[Logger](injector, "file")

	consoleLogger.Log("To console")
	fileLogger.Log("To file")
}

// ====================
// Example 3: Instance Binding
// ====================
func ExampleInstanceBinding() {
	injector, _ := NewInjector()

	// Create and configure instances
	db := &PostgresDB{connectionString: "localhost"}
	logger := &FileLogger{filename: "/var/log/app.log"}

	// Bind instances with options
	BindInstance[Database](injector, db, AsSingleton())
	BindInstance[Logger](injector, logger, WithAnnotation("file"))

	// Retrieve instances
	retrievedDB, _ := GetInstance[Database](injector)
	retrievedDB.Query("SELECT 1")
}

// ====================
// Example 4: Provider Binding
// ====================
func ExampleProviderBinding() {
	injector, _ := NewInjector()

	// Simple provider
	BindProvider[Logger](injector, func() Logger {
		return &ConsoleLogger{}
	}, AsSingleton())

	// Provider with dependencies (auto-injected)
	BindInstance[Database](injector, &PostgresDB{})
	BindProviderFunc[Cache](injector, func(db Database) Cache {
		// db is automatically injected!
		return &RedisCache{}
	})

	cache, _ := GetInstance[Cache](injector)
	cache.Set("key", "value")
}

// ====================
// Example 5: Provider with Error Handling
// ====================
func ExampleProviderWithError() {
	injector, _ := NewInjector()

	BindProviderWithError[Database](injector, func() (Database, error) {
		// Could return error if connection fails
		return &PostgresDB{connectionString: "localhost"}, nil
	}, AsEagerSingleton())

	db, err := GetInstance[Database](injector)
	if err != nil {
		panic(err)
	}
	db.Query("SELECT * FROM users")
}

// ====================
// Example 6: Multi-Bindings
// ====================
func ExampleMultiBindings() {
	injector, _ := NewInjector()

	// Register multiple plugins - simple and clean!
	BindMulti[Plugin, *PluginA](injector)
	BindMulti[Plugin, *PluginB](injector)

	// With annotations
	BindMulti[Plugin, *PluginA](injector, WithAnnotation("production"))
	BindMulti[Plugin, *PluginB](injector, WithAnnotation("production"))

	// Retrieve all plugins
	plugins, _ := GetMultiInstance[Plugin](injector)
	for _, plugin := range plugins {
		plugin.Initialize()
	}

	// Or retrieve only production plugins
	prodPlugins, _ := GetMultiAnnotatedInstance[Plugin](injector, "production")
	for _, plugin := range prodPlugins {
		plugin.Execute()
	}
}

// ====================
// Example 7: Map Bindings
// ====================
func ExampleMapBindings() {
	injector, _ := NewInjector()

	// Register databases with keys
	BindMap[Database, *PostgresDB](injector, "primary", AsSingleton())
	BindMap[Database, *PostgresDB](injector, "replica")
	BindMap[Database, *MySQLDB](injector, "analytics")

	// Retrieve all as map
	databases, _ := GetMapInstance[Database](injector)
	databases["primary"].Query("INSERT ...")

	// Or retrieve individual by key
	analyticsDB, _ := GetMapKey[Database](injector, "analytics")
	analyticsDB.Query("SELECT COUNT(*) ...")
}

// ====================
// Example 8: Struct Field Injection
// ====================
func ExampleStructFieldInjection() {
	injector, _ := NewInjector()

	// Set up bindings
	Bind[Logger, *ConsoleLogger](injector)
	Bind[Database, *PostgresDB](injector, AsSingleton())
	BindInstance[Cache](injector, &RedisCache{})

	// Define service with injected dependencies
	type UserService struct {
		Logger Logger   `inject:""`
		DB     Database `inject:""`
		Cache  Cache    `inject:""`
	}

	// Create and inject
	service := &UserService{}
	RequestInjection(injector, service)

	service.Logger.Log("UserService initialized")
	service.DB.Query("SELECT * FROM users")
}

// ====================
// Example 9: Override Bindings (Testing)
// ====================
func ExampleOverrideBindings() {
	injector, _ := NewInjector()

	// Original production binding
	Bind[Database, *PostgresDB](injector, AsSingleton())

	// Override with mock in tests
	type MockDB struct{}

	func (m *MockDB) Query(sql string) error { return nil }

	Override[Database, *MockDB](injector, "")

	// GetInstance now returns the mock
	db, _ := GetInstance[Database](injector)
	_ = db // This is MockDB
}

// ====================
// Example 10: Interceptors (AOP)
// ====================
func ExampleInterceptors() {
	injector, _ := NewInjector()

	// Define interceptor
	type LoggingInterceptor struct {
		Target Database
		Logger Logger
	}

	func (l *LoggingInterceptor) Query(sql string) error {
		l.Logger.Log("Executing: " + sql)
		return l.Target.Query(sql)
	}

	// Set up bindings
	Bind[Logger, *ConsoleLogger](injector)
	Bind[Database, *PostgresDB](injector)
	BindInterceptor[Database](injector, LoggingInterceptor{})

	// Database is wrapped with logging
	db, _ := GetInstance[Database](injector)
	db.Query("SELECT * FROM users") // Logs before executing
}

// ====================
// Example 11: Complex Provider Dependencies
// ====================
func ExampleComplexProviderDependencies() {
	injector, _ := NewInjector()

	Bind[Logger, *ConsoleLogger](injector, AsSingleton())
	Bind[Database, *PostgresDB](injector, AsSingleton())

	// Provider with multiple auto-injected dependencies
	BindProviderFunc[Cache](injector, func(logger Logger, db Database) Cache {
		logger.Log("Creating cache with database backing")
		return &RedisCache{}
	}, AsSingleton())

	cache, _ := GetInstance[Cache](injector)
	cache.Set("key", "value")
}

// ====================
// Example 12: Full Application
// ====================
func ExampleFullApplication() {
	injector, _ := NewInjector(ModuleFunc(func(i *Injector) {
		// Infrastructure layer
		Bind[Logger, *ConsoleLogger](i, AsSingleton())

		BindProviderWithError[Database](i, func() (Database, error) {
			return &PostgresDB{connectionString: "localhost"}, nil
		}, AsSingleton())

		BindInstance[Cache](i, &RedisCache{}, AsSingleton())

		// Service layer
		type UserRepository struct {
			DB Database `inject:""`
		}

		type UserService struct {
			Repo   *UserRepository `inject:""`
			Logger Logger          `inject:""`
			Cache  Cache           `inject:""`
		}

		// Plugin system
		BindMulti[Plugin, *PluginA](i)
		BindMulti[Plugin, *PluginB](i)
	}))

	// Application startup
	type Application struct {
		Plugins []Plugin `inject:""`
	}

	app := &Application{}
	MustRequestInjection(injector, app)

	for _, plugin := range app.Plugins {
		plugin.Initialize()
	}
}

// ====================
// Example 13: API Comparison
// ====================
func ExampleAPIComparison() {
	injector, _ := NewInjector()

	// ============ BASIC BINDING ============
	// Old API:
	// injector.Bind((*Logger)(nil)).To(ConsoleLogger{})

	// Previous generic API:
	// Bind[Logger](injector).To(&ConsoleLogger{})

	// New API:
	Bind[Logger, *ConsoleLogger](injector)

	// ============ WITH OPTIONS ============
	// Old API:
	// injector.Bind((*Logger)(nil)).To(ConsoleLogger{}).AnnotatedWith("console").In(Singleton)

	// Previous generic API:
	// Bind[Logger](injector).To(&ConsoleLogger{}).AnnotatedWith("console").In(Singleton)

	// New API:
	Bind[Logger, *ConsoleLogger](injector, WithAnnotation("console"), AsSingleton())

	// ============ INSTANCE BINDING ============
	logger := &ConsoleLogger{}

	// Old API:
	// injector.Bind((*Logger)(nil)).ToInstance(logger)

	// Previous generic API:
	// BindInstance[Logger](injector, logger)

	// New API (same, but with options):
	BindInstance[Logger](injector, logger, AsSingleton())

	// ============ MULTI-BINDING ============
	// Old API:
	// injector.BindMulti((*Plugin)(nil)).To(PluginA{})

	// Previous generic API:
	// BindMulti[Plugin](injector).To(&PluginA{})

	// New API:
	BindMulti[Plugin, *PluginA](injector)

	// ============ MAP BINDING ============
	// Old API:
	// injector.BindMap((*Database)(nil), "primary").To(PostgresDB{})

	// Previous generic API:
	// BindMap[Database](injector, "primary").To(&PostgresDB{})

	// New API:
	BindMap[Database, *PostgresDB](injector, "primary")

	// The new API is cleaner and more concise!
	_ = injector
	_ = logger
}

// ====================
// Example 14: Functional Options Pattern
// ====================
func ExampleFunctionalOptions() {
	injector, _ := NewInjector()

	// Multiple options can be combined
	Bind[Logger, *ConsoleLogger](injector,
		WithAnnotation("prod"),
		AsSingleton(),
	)

	// Options work with all binding types
	BindInstance[Database](injector, &PostgresDB{},
		WithAnnotation("primary"),
		WithScope(Singleton),
	)

	BindProvider[Cache](injector, func() Cache {
		return &RedisCache{}
	},
		WithAnnotation("fast"),
		AsEagerSingleton(),
	)

	BindMulti[Plugin, *PluginA](injector,
		WithAnnotation("essential"),
	)

	BindMap[Database, *MySQLDB](injector, "analytics",
		WithAnnotation("reporting"),
		AsChildSingleton(),
	)

	// Clean, extensible, and type-safe!
}

// ====================
// Example 15: Compatibility
// ====================
func ExampleCompatibility() {
	injector, _ := NewInjector()

	// Mix old and new API freely!

	// Old reflection-based API
	injector.Bind((*Logger)(nil)).To(ConsoleLogger{})

	// New generic API
	Bind[Database, *PostgresDB](injector)

	// Both work together seamlessly
	loggerInterface, _ := injector.GetInstance((*Logger)(nil))
	logger := loggerInterface.(Logger)

	db, _ := GetInstance[Database](injector) // No type assertion!

	logger.Log("Mixed API usage works perfectly")
	db.Query("SELECT 1")
}

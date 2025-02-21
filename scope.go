package dingo

import (
	"sync"

	"flamingo.me/dingo/internal"
)

type (
	// Scope defines a scope's behaviour
	Scope               = internal.Scope
	SingletonScope      = internal.SingletonScope
	ChildSingletonScope = internal.ChildSingletonScope
)

var (
	mu sync.Mutex
	// Singleton is the default SingletonScope for dingo
	Singleton = internal.Singleton()

	// ChildSingleton is a per-child singleton, means singletons are scoped and local to an injector instance
	ChildSingleton = internal.ChildSingleton()

	// NewSingletonScope creates a new singleton scope
	NewSingletonScope = internal.NewSingletonScope
	// NewChildSingletonScope creates a new child singleton scope
	NewChildSingletonScope = internal.NewChildSingletonScope
)

func ResetScope() {
	mu.Lock()
	defer mu.Unlock()

	internal.Reset()
	Singleton = internal.Singleton()
	ChildSingleton = internal.ChildSingleton()
}

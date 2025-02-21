package internal

import (
	"reflect"
	"sync"
	"sync/atomic"
)

var (
	singleton      = defaultSingletonScope()
	childSingleton = defaultChildSingletonScope()
)

func defaultSingletonScope() *atomic.Value {
	v := &atomic.Value{}
	v.Store(NewSingletonScope())
	return v
}

func defaultChildSingletonScope() *atomic.Value {
	v := &atomic.Value{}
	v.Store(NewChildSingletonScope())
	return v
}

func Singleton() Scope {
	return singleton.Load().(*SingletonScope)
}

func ChildSingleton() Scope {
	return childSingleton.Load().(*ChildSingletonScope)
}

func Reset() {
	singleton.Store(singleton.Load().(*SingletonScope).Reset())
	childSingleton.Store(childSingleton.Load().(*ChildSingletonScope).Reset())
}

type (
	// Scope defines a scope's behaviour
	Scope interface {
		ResolveType(t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error)) (reflect.Value, error)
	}

	identifier struct {
		t reflect.Type
		a string
	}

	// SingletonScope is our Scope to handle Singletons
	// todo use RWMutex for proper locking
	SingletonScope struct {
		mu           sync.Mutex                   // lock guarding instanceLocks
		instanceLock map[identifier]*sync.RWMutex // lock guarding instances
		instances    sync.Map
	}

	// ChildSingletonScope manages child-specific singleton
	ChildSingletonScope SingletonScope
)

// NewSingletonScope creates a new singleton scope
func NewSingletonScope() *SingletonScope {
	return &SingletonScope{instanceLock: make(map[identifier]*sync.RWMutex)}
}

// NewChildSingletonScope creates a new child singleton scope
func NewChildSingletonScope() *ChildSingletonScope {
	return &ChildSingletonScope{instanceLock: make(map[identifier]*sync.RWMutex)}
}

// ResolveType resolves a request in this scope
func (s *SingletonScope) ResolveType(t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error)) (reflect.Value, error) {
	ident := identifier{t, annotation}

	// try to get the instance type lock
	s.mu.Lock()

	if l, ok := s.instanceLock[ident]; ok {
		// we have the instance lock
		s.mu.Unlock()
		l.RLock()
		defer l.RUnlock()

		instance, _ := s.instances.Load(ident)
		return instance.(reflect.Value), nil
	}

	s.instanceLock[ident] = new(sync.RWMutex)
	l := s.instanceLock[ident]
	l.Lock()
	s.mu.Unlock()

	instance, err := unscoped(t, annotation, false)
	s.instances.Store(ident, instance)

	defer l.Unlock()

	return instance, err
}

// ResolveType delegates to SingletonScope.ResolveType
func (c *ChildSingletonScope) ResolveType(t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error)) (reflect.Value, error) {
	return (*SingletonScope)(c).ResolveType(t, annotation, unscoped)
}

func (s *SingletonScope) Reset() *SingletonScope {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.instanceLock = make(map[identifier]*sync.RWMutex)
	s.instances.Clear()

	return s
}

func (c *ChildSingletonScope) Reset() *ChildSingletonScope {
	return (*ChildSingletonScope)((*SingletonScope)(c).Reset())
}

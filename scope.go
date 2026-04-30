package dingo

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

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
	SingletonScope struct {
		mu           sync.Mutex
		instanceLock map[identifier]*sync.Mutex
		instances    sync.Map
		// creating tracks which goroutines are currently creating which instances.
		// Outer key: identifier, inner key: goroutine ID, value: true.
		creating sync.Map // map[identifier]*sync.Map
	}

	// ChildSingletonScope manages child-specific singleton
	ChildSingletonScope SingletonScope
)

var (
	// Singleton is the default SingletonScope for dingo
	Singleton Scope = NewSingletonScope()

	// ChildSingleton is a per-child singleton, means singletons are scoped and local to an injector instance
	ChildSingleton Scope = NewChildSingletonScope()
)

// goroutineID returns the current goroutine\'s ID from the runtime stack.
func goroutineID() string {
	var buf [128]byte
	n := runtime.Stack(buf[:], false)
	s := string(buf[:n])
	if i := strings.Index(s, " "); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, " "); i >= 0 {
		return s[:i]
	}
	return s
}

// isCreating checks if the given goroutine is currently creating the given identifier.
func (s *SingletonScope) isCreating(ident identifier, gid string) bool {
	if val, ok := s.creating.Load(ident); ok {
		inner, ok := val.(*sync.Map) //nolint:forcetypeassert // creating always stores *sync.Map values
		_, loaded := inner.Load(gid)

		return loaded && ok
	}

	return false
}

// markCreating marks the given goroutine as creating the given identifier.
func (s *SingletonScope) markCreating(ident identifier, gid string) {
	val, _ := s.creating.LoadOrStore(ident, &sync.Map{})
	inner, _ := val.(*sync.Map) //nolint:forcetypeassert // creating always stores *sync.Map values
	inner.Store(gid, true)
}

// unmarkCreating removes the mark. Cleans up the inner map if empty.
func (s *SingletonScope) unmarkCreating(ident identifier, gid string) {
	if val, ok := s.creating.Load(ident); ok {
		inner, _ := val.(*sync.Map) //nolint:forcetypeassert // creating always stores *sync.Map values
		inner.Delete(gid)
	}
}

// NewSingletonScope creates a new singleton scope
func NewSingletonScope() *SingletonScope {
	return &SingletonScope{instanceLock: make(map[identifier]*sync.Mutex)}
}

// NewChildSingletonScope creates a new child-singleton scope
func NewChildSingletonScope() *ChildSingletonScope {
	return &ChildSingletonScope{instanceLock: make(map[identifier]*sync.Mutex)}
}

// ResolveType resolves a request in this scope
func (s *SingletonScope) ResolveType(t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error)) (reflect.Value, error) {
	ident := identifier{t, annotation}
	gid := goroutineID()

	// Check if THIS goroutine is already creating this instance (circular dependency).
	// Must happen BEFORE acquiring any lock to detect the cycle before deadlock.
	if s.isCreating(ident, gid) {
		panic(fmt.Sprintf("detected circular singleton dependency for %s", t))
	}

	s.mu.Lock()

	if l, ok := s.instanceLock[ident]; ok {
		s.mu.Unlock()
		l.Lock()
		defer l.Unlock()

		if instance, ok := s.instances.Load(ident); ok {
			return instance.(reflect.Value), nil
		}
		// Lock acquired but no instance — previous creator panicked or errored.
		// We own the lock now, fall through to create.
	}

	l := new(sync.Mutex)
	s.instanceLock[ident] = l
	l.Lock()
	s.mu.Unlock()

	// Mark this goroutine as creating this instance
	s.markCreating(ident, gid)
	defer s.unmarkCreating(ident, gid)

	defer l.Unlock()

	instance, err := unscoped(t, annotation, false)
	if err != nil {
		return reflect.Value{}, err
	}
	s.instances.Store(ident, instance)

	return instance, nil
}

// ResolveType delegates to SingletonScope.ResolveType
func (c *ChildSingletonScope) ResolveType(t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error)) (reflect.Value, error) {
	return (*SingletonScope)(c).ResolveType(t, annotation, unscoped)
}

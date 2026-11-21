package contract_test

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingScope is a user-supplied scope: it caches per type and counts how
// often it had to construct.
type countingScope struct {
	constructed int
	instances   map[reflect.Type]reflect.Value
}

func (c *countingScope) ResolveType(t reflect.Type, annotation string, unscoped func(t reflect.Type, annotation string, optional bool) (reflect.Value, error)) (reflect.Value, error) {
	if instance, ok := c.instances[t]; ok {
		return instance, nil
	}

	instance, err := unscoped(t, annotation, false)
	if err != nil {
		return reflect.Value{}, err
	}

	c.constructed++
	c.instances[t] = instance

	return instance, nil
}

func newCountingScope() *countingScope {
	return &countingScope{instances: make(map[reflect.Type]reflect.Value)}
}

func TestScope_SingletonCachesWithinOneInjector(t *testing.T) {
	t.Parallel()

	scoped := newInjector(t)
	scoped.Bind(new(counter)).In(dingo.Singleton)

	first, err := scoped.GetInstance(new(counter))
	require.NoError(t, err)

	second, err := scoped.GetInstance(new(counter))
	require.NoError(t, err)

	assert.Same(t, first, second)

	unscoped := newInjector(t)
	unscoped.Bind(new(counter))

	third, err := unscoped.GetInstance(new(counter))
	require.NoError(t, err)

	fourth, err := unscoped.GetInstance(new(counter))
	require.NoError(t, err)

	assert.NotSame(t, third, fourth)
}

func TestScope_ChildSingletonCachesPerChild(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	parent.Bind(new(counter)).In(dingo.ChildSingleton)

	first := parent.child(t)
	second := parent.child(t)

	firstInstance, err := first.GetInstance(new(counter))
	require.NoError(t, err)

	firstAgain, err := first.GetInstance(new(counter))
	require.NoError(t, err)

	secondInstance, err := second.GetInstance(new(counter))
	require.NoError(t, err)

	assert.Same(t, firstInstance, firstAgain)
	assert.NotSame(t, firstInstance, secondInstance)
}

func TestScope_CustomScopeResolvesWhenRegistered(t *testing.T) {
	t.Parallel()

	scope := newCountingScope()

	injector := newInjector(t)
	injector.BindScope(scope)
	injector.Bind(new(counter)).In(scope)

	first, err := injector.GetInstance(new(counter))
	require.NoError(t, err)

	second, err := injector.GetInstance(new(counter))
	require.NoError(t, err)

	assert.Same(t, first, second)
	assert.Equal(t, 1, scope.constructed)
}

func TestScope_UnregisteredScopeErrors(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.Bind(new(counter)).In(newCountingScope())

	_, err := injector.GetInstance(new(counter))

	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown scope")
	assert.ErrorContains(t, err, "countingScope")
}

func TestBindScope_FreshScopeReplacesTheCache(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: scopes are registered by Go type,
	// so a fresh object of the same type replaces the registration and the
	// cache behind it. Do not derive a child from this injector afterwards;
	// the helper still holds the scope it registered.
	injector := newInjector(t)
	injector.Bind(new(counter)).In(dingo.Singleton)

	before, err := injector.GetInstance(new(counter))
	require.NoError(t, err)

	injector.BindScope(dingo.NewSingletonScope())

	after, err := injector.GetInstance(new(counter))
	require.NoError(t, err)

	assert.NotSame(t, before, after)
}

// processSingleton is resolved from the process-wide singleton scope.
type processSingleton struct {
	N int
}

// reassignedSingleton is resolved after the process-wide scopes were replaced.
type reassignedSingleton struct {
	N int
}

// Injectors that use the process-wide dingo.Singleton share its cache, so two
// injectors in one process get the same instance.
func TestLatent_SingletonIsSharedByTheWholeProcess(t *testing.T) {
	t.Parallel()

	resolve := func() any {
		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		injector.Bind(new(processSingleton)).In(dingo.Singleton)

		instance, err := injector.GetInstance(new(processSingleton))
		require.NoError(t, err)

		return instance
	}

	assert.Same(t, resolve(), resolve())
}

// A program can replace dingo.Singleton and dingo.ChildSingleton with new
// scopes. Injectors made afterwards then use new caches.
func TestScope_ReassignedGlobalScopesIsolateTheCaches(t *testing.T) { //nolint:paralleltest // replaces the process-wide scopes
	singleton, childSingleton := dingo.Singleton, dingo.ChildSingleton

	t.Cleanup(func() {
		dingo.Singleton, dingo.ChildSingleton = singleton, childSingleton
	})

	resolve := func(scope dingo.Scope) any {
		injector, err := dingo.NewInjector()
		require.NoError(t, err)

		injector.Bind(new(reassignedSingleton)).In(scope)

		instance, err := injector.GetInstance(new(reassignedSingleton))
		require.NoError(t, err)

		return instance
	}

	before := resolve(dingo.Singleton)
	dingo.Singleton = dingo.NewSingletonScope()
	assert.NotSame(t, before, resolve(dingo.Singleton))

	before = resolve(dingo.ChildSingleton)
	dingo.ChildSingleton = dingo.NewChildSingletonScope()
	assert.NotSame(t, before, resolve(dingo.ChildSingleton))
}

// Many goroutines that resolve a scoped binding for the first time at once
// get one instance, and the instance is built once.
func TestScope_ConcurrentFirstResolutionBuildsOneInstance(t *testing.T) {
	t.Parallel()

	const goroutines = 16

	for name, scope := range map[string]dingo.Scope{
		"singleton":       dingo.Singleton,
		"child singleton": dingo.ChildSingleton,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var built atomic.Int32

			injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
				injector.Bind(new(counter)).ToProvider(func() *counter {
					built.Add(1)
					time.Sleep(10 * time.Millisecond)

					return &counter{}
				}).In(scope)
			}))

			var start, done sync.WaitGroup

			start.Add(1)

			results := make([]any, goroutines)

			for index := range goroutines {
				done.Go(func() {
					start.Wait()

					instance, err := injector.GetInstance(new(counter))
					assert.NoError(t, err)

					results[index] = instance
				})
			}

			start.Done()
			done.Wait()

			assert.Equal(t, int32(1), built.Load())

			for _, result := range results {
				assert.Same(t, results[0], result)
			}
		})
	}
}

package dingo_test

import (
	"reflect"
	"testing"

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

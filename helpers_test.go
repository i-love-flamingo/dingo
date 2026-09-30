package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolatedInjector is an injector plus the singleton scope object registered
// on it.
//
// dingo.Singleton is one process-wide object that every injector registers, so
// two injectors that both resolve an In(dingo.Singleton) binding of the same
// type share one instance cache — and the cache keeps an instance even when
// its construction failed. Tests that assert instance identity therefore
// register their own *dingo.SingletonScope: scopes are looked up by Go type,
// so In(dingo.Singleton) keeps selecting the singleton scope and resolves
// through the private cache.
//
// Derive children with child(t), never with the promoted Child(): the
// promoted method compiles and returns an injector that resolves
// In(dingo.Singleton) in the process-wide cache.
type isolatedInjector struct {
	*dingo.Injector

	singleton dingo.Scope
}

// newInjector returns an injector with a private singleton and child-singleton
// cache, initialized with modules.
//
// The scopes are registered before InitModules runs, so eager singletons are
// built in the private cache too. Call it without modules and call InitModules
// afterwards when the test needs the error: dingo.NewInjector(modules...) is
// dingo.NewInjector() followed by InitModules(modules...).
func newInjector(t *testing.T, modules ...dingo.Module) *isolatedInjector {
	t.Helper()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	singleton := dingo.NewSingletonScope()
	injector.BindScope(singleton)
	injector.BindScope(dingo.NewChildSingletonScope())

	require.NoError(t, injector.InitModules(modules...))

	return &isolatedInjector{Injector: injector, singleton: singleton}
}

// child derives a child injector that keeps the parent's private singleton
// cache. Child() registers the process-wide dingo.Singleton on the new
// injector, which would undo the isolation; the fresh child-singleton scope it
// registers is per child already and is left alone.
//
// Do not call it on an injector whose singleton scope was replaced by a later
// BindScope: this type still holds the scope registered here.
func (i *isolatedInjector) child(t *testing.T) *isolatedInjector {
	t.Helper()

	child, err := i.Child()
	require.NoError(t, err)

	child.BindScope(i.singleton)

	return &isolatedInjector{Injector: child, singleton: i.singleton}
}

type failingEagerCounter struct {
	Missing greeter `inject:""`
}

type failingEagerModule struct{}

func (failingEagerModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(failingEagerCounter)).AsEagerSingleton()
}

func TestHelper_PrivateSingletonScopeIsolatesTwoRoots(t *testing.T) {
	t.Parallel()

	first := newInjector(t)
	first.Bind(new(counter)).In(dingo.Singleton)

	second := newInjector(t)
	second.Bind(new(counter)).In(dingo.Singleton)

	fromFirst, err := first.GetInstance(new(counter))
	require.NoError(t, err)

	fromSecond, err := second.GetInstance(new(counter))
	require.NoError(t, err)

	assert.NotSame(t, fromFirst, fromSecond)
}

func TestHelper_PrivateChildSingletonScopeIsolatesTwoRoots(t *testing.T) {
	t.Parallel()

	first := newInjector(t)
	first.Bind(new(counter)).In(dingo.ChildSingleton)

	second := newInjector(t)
	second.Bind(new(counter)).In(dingo.ChildSingleton)

	fromFirst, err := first.GetInstance(new(counter))
	require.NoError(t, err)

	fromSecond, err := second.GetInstance(new(counter))
	require.NoError(t, err)

	assert.NotSame(t, fromFirst, fromSecond)
}

func TestHelper_ChildSharesTheParentsPrivateSingletonScope(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	parent.Bind(new(counter)).In(dingo.Singleton)

	child := parent.child(t)

	fromParent, err := parent.GetInstance(new(counter))
	require.NoError(t, err)

	fromChild, err := child.GetInstance(new(counter))
	require.NoError(t, err)

	assert.Same(t, fromParent, fromChild)
}

func TestHelper_FailedConstructionInOneRootDoesNotLeakIntoTheNext(t *testing.T) {
	t.Parallel()

	first := newInjector(t)
	assert.Error(t, first.InitModules(failingEagerModule{}))

	second := newInjector(t)
	assert.Error(t, second.InitModules(failingEagerModule{}))
}

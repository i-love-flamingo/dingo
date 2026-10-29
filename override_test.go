package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// equalBindingsModule binds the same target twice. Two To-bindings of one
// target are structurally equal, which duplicate detection tolerates.
type equalBindingsModule struct{}

func (equalBindingsModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(greeter)).To(helloGreeter{})
	injector.Bind(new(greeter)).To(helloGreeter{})
}

// conflictingBindingsModule binds two different targets under one annotation.
// The annotation is not part of either type name, so the error has to name it.
type conflictingBindingsModule struct{}

func (conflictingBindingsModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(greeter)).AnnotatedWith("shouting").To(helloGreeter{})
	injector.Bind(new(greeter)).AnnotatedWith("shouting").To(loudGreeter{})
}

// conflictingInstancesModule binds two different instances under one key.
type conflictingInstancesModule struct{}

func (conflictingInstancesModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("first")
	injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("second")
}

func TestDuplicateBinding_EqualBindingsAreTolerated(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	require.NoError(t, injector.InitModules(equalBindingsModule{}))

	resolved, err := injector.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hello", resolved.(greeter).Greet())
}

func TestDuplicateBinding_UnequalBindingsErrorAtInitModules(t *testing.T) {
	t.Parallel()

	t.Run("to bindings name both targets", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)

		err := injector.InitModules(conflictingBindingsModule{})

		require.Error(t, err)
		assert.ErrorContains(t, err, "already known binding")
		assert.ErrorContains(t, err, "greeter")
		assert.ErrorContains(t, err, "shouting")
		assert.ErrorContains(t, err, "helloGreeter")
		assert.ErrorContains(t, err, "loudGreeter")
	})

	t.Run("instance bindings name the type and annotation", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)

		err := injector.InitModules(conflictingInstancesModule{})

		require.Error(t, err)
		assert.ErrorContains(t, err, "already known binding")
		assert.ErrorContains(t, err, "string")
		assert.ErrorContains(t, err, "greeting")
	})
}

func TestOverride_UnboundKeyActsAsBind(t *testing.T) {
	t.Parallel()

	// Override binds the key itself before the override loop runs, so
	// overriding a key that nothing else bound works like Bind.
	injector := newInjector(t)

	require.NoError(t, injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Override(new(string), "greeting").ToInstance("override")
	})))

	resolved, err := injector.GetAnnotatedInstance(new(string), "greeting")
	require.NoError(t, err)
	assert.Equal(t, "override", resolved)
}

func TestOverride_AfterInitModulesAppliesOnTheNextInitModules(t *testing.T) {
	t.Parallel()

	// Overrides are applied inside InitModules, so one registered afterwards
	// waits for the next call. What resolves before that call is not
	// asserted: applying or reporting the override earlier would be fine.
	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("first")
	}))

	injector.Override(new(string), "greeting").ToInstance("late")
	require.NoError(t, injector.InitModules())

	resolved, err := injector.GetAnnotatedInstance(new(string), "greeting")
	require.NoError(t, err)
	assert.Equal(t, "late", resolved)
}

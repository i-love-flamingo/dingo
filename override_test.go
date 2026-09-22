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
type conflictingBindingsModule struct{}

func (conflictingBindingsModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(greeter)).AnnotatedWith("loud").To(helloGreeter{})
	injector.Bind(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})
}

// conflictingInstancesModule binds two different instances under one key. The
// duplicate error interpolates only the To target, so for instance bindings it
// names empty strings.
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
		assert.ErrorContains(t, err, "loud")
		assert.ErrorContains(t, err, "helloGreeter")
		assert.ErrorContains(t, err, "loudGreeter")
	})

	t.Run("instance bindings name only the key", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)

		err := injector.InitModules(conflictingInstancesModule{})

		require.Error(t, err)
		assert.ErrorContains(t, err, "already known binding")
		assert.ErrorContains(t, err, "string")
		assert.ErrorContains(t, err, "greeting")
	})
}

func TestOverride_ReplacesEveryMatchingAnnotationEntry(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	require.NoError(t, injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("first")
		injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("second")
		injector.Override(new(string), "greeting").ToInstance("override")
	})))

	resolved, err := injector.GetAnnotatedInstance(new(string), "greeting")
	require.NoError(t, err)
	assert.Equal(t, "override", resolved)

	// Resolution reads the first matching entry only, so ask the inspector
	// whether every entry for the key was replaced. The number of entries is
	// deliberately not asserted: Override appends a binding of its own.
	spy := newSpyInspector()
	injector.Inspect(spy.inspector())

	greetings := spy.bindings[typeOf[string]()]
	require.NotEmpty(t, greetings)

	for _, binding := range greetings {
		require.NotNil(t, binding.instance)
		assert.Equal(t, "override", binding.instance.Interface())
	}
}

func TestOverride_UnknownBindingSilentlyBecomesAPlainBinding(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: Override calls Bind first, so the
	// key it is supposed to override always exists by the time the override
	// loop runs and the "cannot override unknown binding" branch is dead.
	injector := newInjector(t)

	require.NoError(t, injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Override(new(string), "greeting").ToInstance("override")
	})))

	resolved, err := injector.GetAnnotatedInstance(new(string), "greeting")
	require.NoError(t, err)
	assert.Equal(t, "override", resolved)
}

func TestOverride_AfterInitModulesIsNeverEvaluated(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: the override loop runs inside
	// InitModules, so a later Override is a plain binding appended behind the
	// one it meant to replace, and nothing reports that.
	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(string)).AnnotatedWith("greeting").ToInstance("first")
	}))

	injector.Override(new(string), "greeting").ToInstance("late")

	resolved, err := injector.GetAnnotatedInstance(new(string), "greeting")
	require.NoError(t, err)
	assert.Equal(t, "first", resolved)
}

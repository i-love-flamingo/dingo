package contract_test

import (
	"reflect"
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

type overrideTarget struct{ name string }

type overrideTargetHolder struct {
	Plain     *overrideTarget `inject:""`
	Annotated *overrideTarget `inject:"special"`
}

// Lookups resolve *T through the T binding, so Override(new(*T)) replaces the
// T binding.
func TestOverride_PointerTypeReplacesTheValueBinding(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(overrideTarget)).ToInstance(&overrideTarget{name: "original"})
		injector.Bind(new(overrideTarget)).AnnotatedWith("special").ToInstance(&overrideTarget{name: "original special"})
	}), dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Override(new(*overrideTarget), "").ToInstance(&overrideTarget{name: "override"})
		injector.Override(new(*overrideTarget), "special").ToInstance(&overrideTarget{name: "override special"})
	}))

	resolved, err := injector.GetInstance(new(overrideTarget))
	require.NoError(t, err)
	assert.Equal(t, "override", resolved.(*overrideTarget).name)

	resolved, err = injector.GetAnnotatedInstance(new(overrideTarget), "special")
	require.NoError(t, err)
	assert.Equal(t, "override special", resolved.(*overrideTarget).name)

	var holder overrideTargetHolder
	require.NoError(t, injector.RequestInjection(&holder))
	assert.Equal(t, "override", holder.Plain.name)
	assert.Equal(t, "override special", holder.Annotated.name)
}

func TestOverride_PointerTypeWithOnlyThePointerBoundErrors(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	err := injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(*overrideTarget)).ToInstance(&overrideTarget{name: "original"})
		injector.Override(new(*overrideTarget), "").ToInstance(&overrideTarget{name: "override"})
	}))

	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot override unknown binding")
}

func TestOverride_PointerTypeWithValueAndPointerBoundErrors(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	err := injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(overrideTarget)).ToInstance(&overrideTarget{name: "value"})
		injector.Bind(new(*overrideTarget)).ToInstance(&overrideTarget{name: "pointer"})
		injector.Override(new(*overrideTarget), "").ToInstance(&overrideTarget{name: "override"})
	}))

	require.Error(t, err)
	assert.ErrorContains(t, err, "already known binding")
}

func TestOverride_PointerTypeInAChildReplacesOnlyTheChildBinding(t *testing.T) {
	t.Parallel()

	parent := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(overrideTarget)).ToInstance(&overrideTarget{name: "parent"})
	}))

	t.Run("without a child binding it errors", func(t *testing.T) {
		t.Parallel()

		err := parent.child(t).InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Override(new(*overrideTarget), "").ToInstance(&overrideTarget{name: "override"})
		}))

		require.Error(t, err)
		assert.ErrorContains(t, err, "cannot override unknown binding")
	})

	t.Run("with a child binding it replaces it", func(t *testing.T) {
		t.Parallel()

		child := parent.child(t)
		require.NoError(t, child.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(overrideTarget)).ToInstance(&overrideTarget{name: "child"})
			injector.Override(new(*overrideTarget), "").ToInstance(&overrideTarget{name: "override"})
		})))

		resolved, err := child.GetInstance(new(overrideTarget))
		require.NoError(t, err)
		assert.Equal(t, "override", resolved.(*overrideTarget).name)

		resolved, err = parent.GetInstance(new(overrideTarget))
		require.NoError(t, err)
		assert.Equal(t, "parent", resolved.(*overrideTarget).name)
	})
}

// Override strips exactly one pointer more than Bind, so Override(new(**T))
// replaces the *T binding, which only a reflect.Type lookup of **T reads.
func TestOverride_DoublePointerTypeReplacesThePointerBinding(t *testing.T) {
	t.Parallel()

	t.Run("with the pointer bound it replaces it", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(*overrideTarget)).ToInstance(&overrideTarget{name: "pointer"})
		}), dingo.ModuleFunc(func(injector *dingo.Injector) {
			override := &overrideTarget{name: "override"}
			injector.Override(new(**overrideTarget), "").ToInstance(&override)
		}))

		resolved, err := injector.GetInstance(reflect.TypeFor[**overrideTarget]())
		require.NoError(t, err)
		assert.Equal(t, "override", (*resolved.(**overrideTarget)).name)
	})

	t.Run("with only the value bound it errors", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)

		err := injector.InitModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(overrideTarget)).ToInstance(&overrideTarget{name: "value"})

			override := &overrideTarget{name: "override"}
			injector.Override(new(**overrideTarget), "").ToInstance(&override)
		}))

		require.Error(t, err)
		assert.ErrorContains(t, err, "cannot override unknown binding")
	})
}

func TestLatent_BindPointerTypeIsNotReadByLookups(t *testing.T) {
	t.Parallel()

	// Known quirk: Bind strips one pointer and lookups strip one more, so a
	// binding for *T is never found by T or *T lookups, and they build a zero
	// T instead. Only a reflect.Type lookup of **T reads it.
	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(*overrideTarget)).ToInstance(&overrideTarget{name: "pointer"})
	}))

	resolved, err := injector.GetInstance(new(overrideTarget))
	require.NoError(t, err)
	assert.Empty(t, resolved.(*overrideTarget).name)

	var holder struct {
		Plain *overrideTarget `inject:""`
	}
	require.NoError(t, injector.RequestInjection(&holder))
	assert.Empty(t, holder.Plain.name)
}

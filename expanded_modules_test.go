package dingo_test

import (
	"reflect"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type selfDependentModule struct {
	dependency dingo.Module
}

func (*selfDependentModule) Configure(*dingo.Injector) {}

func (module *selfDependentModule) Depends() []dingo.Module {
	return []dingo.Module{module.dependency}
}

func TestModuleSelfDependencies(t *testing.T) {
	t.Parallel()

	for _, sameInstance := range []bool{true, false} {
		name := "same type"
		if sameInstance {
			name = "same instance"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			module := &selfDependentModule{dependency: &selfDependentModule{}}
			if sameInstance {
				module.dependency = module
			}

			t.Run("ExpandModules", func(t *testing.T) {
				t.Parallel()

				var err error

				require.NotPanics(t, func() {
					_, err = dingo.ExpandModules(module)
				})
				assert.ErrorIs(t, err, dingo.ErrModuleCycle)
				assert.ErrorContains(t, err, "selfDependentModule")
			})

			t.Run("InitModules", func(t *testing.T) {
				t.Parallel()

				injector, err := dingo.NewInjector()
				require.NoError(t, err)
				require.NotPanics(t, func() {
					err = injector.InitModules(module)
				})
				assert.ErrorIs(t, err, dingo.ErrModuleCycle)
				assert.ErrorIs(t, err, dingo.ErrInitModules)
				assert.ErrorContains(t, err, "selfDependentModule")
			})
		})
	}
}

type capturedDependencyModule struct {
	dependsCalls int
	configured   *[]string
	value        int
}

func (module *capturedDependencyModule) Depends() []dingo.Module {
	module.dependsCalls++
	value := module.dependsCalls

	return []dingo.Module{dingo.ModuleFunc(func(injector *dingo.Injector) {
		*module.configured = append(*module.configured, "dependency")

		injector.Bind(new(int)).ToInstance(value)
	})}
}

func (module *capturedDependencyModule) Inject(value int) {
	module.value = value
}

func (module *capturedDependencyModule) Configure(*dingo.Injector) {
	*module.configured = append(*module.configured, "parent")
}

func (*capturedDependencyModule) Config() string { return "parent config" }

func TestInitExpandedModulesPreservesSnapshot(t *testing.T) {
	t.Parallel()

	var configured []string

	parent := &capturedDependencyModule{configured: &configured}
	modules, err := dingo.ExpandModules(parent)
	require.NoError(t, err)
	require.Len(t, modules, 2)
	assert.Same(t, parent, modules[1])
	configurable, ok := modules[1].(interface{ Config() string })
	require.True(t, ok)
	assert.Equal(t, "parent config", configurable.Config())

	for range 2 {
		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		require.NoError(t, injector.InitExpandedModules(modules...))
		assert.Equal(t, 1, parent.value, "module injection must use the captured dependency")

		instance, err := injector.GetInstance(new(int))
		require.NoError(t, err)
		assert.Equal(t, 1, instance)
	}

	assert.Equal(t, 1, parent.dependsCalls, "initialization must not expand dependencies again")
	assert.Equal(t, []string{"dependency", "parent", "dependency", "parent"}, configured)
}

type orderedModule struct {
	name       string
	configured *[]string
}

func (module *orderedModule) Configure(*dingo.Injector) {
	*module.configured = append(*module.configured, module.name)
}

func (*orderedModule) Depends() []dingo.Module { panic("unexpected expansion") }

func TestInitExpandedModulesKeepsFirstIdentityAndInputOrder(t *testing.T) {
	t.Parallel()

	var configured []string

	first := &orderedModule{name: "first ordinary", configured: &configured}
	second := &orderedModule{name: "second ordinary", configured: &configured}
	modules := make([]dingo.Module, 0, 6)
	modules = append(modules, first)

	for _, name := range []string{"first closure", "second closure"} {
		modules = append(modules, dingo.ModuleFunc(func(*dingo.Injector) {
			configured = append(configured, name)
		}))
	}

	modules = append(modules, second, modules[1], first)
	original := append([]dingo.Module(nil), modules...)
	injector, err := dingo.NewInjector()
	require.NoError(t, err)
	require.NoError(t, injector.InitExpandedModules(modules...))

	assert.Equal(t, []string{"first ordinary", "first closure", "second closure"}, configured)

	for index := range original {
		assert.Equal(t, reflect.ValueOf(original[index]), reflect.ValueOf(modules[index]), "input at %d changed", index)
	}
}

type expandedRequest struct {
	value string
}

func (request *expandedRequest) Inject(value string) { request.value = value }

func TestInitExpandedModulesFinalizesBindings(t *testing.T) {
	t.Parallel()

	t.Run("deferred requests use the final override", func(t *testing.T) {
		t.Parallel()

		request := &expandedRequest{}
		requesting := dingo.ModuleFunc(func(injector *dingo.Injector) {
			require.NoError(t, injector.RequestInjection(request))
			assert.Empty(t, request.value)
		})
		binding := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(string)).ToInstance("original")
		})
		override := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Override(new(string), "").ToInstance("replacement")
		})
		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		require.NoError(t, injector.InitExpandedModules(requesting, binding, override))
		assert.Equal(t, "replacement", request.value)
	})

	t.Run("duplicate bindings are rejected", func(t *testing.T) {
		t.Parallel()

		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		err = injector.InitExpandedModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(string)).ToInstance("first")
			injector.Bind(new(string)).ToInstance("second")
		}))
		assert.ErrorContains(t, err, "already known binding")
	})

	t.Run("eager singletons are constructed once", func(t *testing.T) {
		t.Parallel()

		calls := 0
		injector, err := dingo.NewInjector()
		require.NoError(t, err)
		require.NoError(t, injector.InitExpandedModules(dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(string)).ToProvider(func() string {
				calls++
				return "eager"
			}).AsEagerSingleton()
		})))
		assert.Equal(t, 1, calls)

		instance, err := injector.GetInstance(new(string))
		require.NoError(t, err)
		assert.Equal(t, "eager", instance)
		assert.Equal(t, 1, calls)
	})
}

func TestTryExpandedModules(t *testing.T) {
	t.Parallel()

	t.Run("uses the supplied list and skips eager construction", func(t *testing.T) {
		t.Parallel()

		var configured []string

		calls := 0
		module := dingo.ModuleFunc(func(injector *dingo.Injector) {
			injector.Bind(new(string)).ToProvider(func() string {
				calls++
				return "eager"
			}).AsEagerSingleton()
		})
		require.NoError(t, dingo.TryExpandedModules(&orderedModule{name: "ordinary", configured: &configured}, module))
		assert.Equal(t, []string{"ordinary"}, configured)
		assert.Zero(t, calls)
	})

	t.Run("recovers error panics", func(t *testing.T) {
		t.Parallel()

		err := dingo.TryExpandedModules(dingo.ModuleFunc(func(*dingo.Injector) {
			panic(dingo.ErrModuleCycle)
		}))
		assert.ErrorIs(t, err, dingo.ErrModuleCycle)
	})

	t.Run("recovers string panics", func(t *testing.T) {
		t.Parallel()

		err := dingo.TryExpandedModules(dingo.ModuleFunc(func(*dingo.Injector) {
			panic("configuration failed")
		}))
		assert.ErrorContains(t, err, "configuration failed")
	})
}

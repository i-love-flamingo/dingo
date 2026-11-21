package contract_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedModule records its name in a shared list when it is configured, and
// adds it to a multibinding, so that both orders can be compared.
type namedModule struct {
	name         string
	order        *[]string
	dependencies []dingo.Module
}

func (module *namedModule) Configure(injector *dingo.Injector) {
	*module.order = append(*module.order, module.name)

	injector.BindMulti(new(string)).AnnotatedWith("order").ToInstance(module.name)
}

func (module *namedModule) Depends() []dingo.Module {
	return module.dependencies
}

// The wrapper types give four module identities that share one
// implementation.
type (
	catalogModule struct{ *namedModule }
	basketModule  struct{ *namedModule }
	brandModule   struct{ *namedModule }
	shopModule    struct{ *namedModule }
)

// Modules that nothing orders against each other configure in the order in
// which InitModules first reaches them: through the list, and depth-first
// through Depends. A dependency always configures before its dependents.
func TestModules_DependenciesConfigureBeforeDependents(t *testing.T) {
	t.Parallel()

	var names []string

	catalog := &catalogModule{&namedModule{name: "catalog", order: &names}}
	basket := &basketModule{&namedModule{name: "basket", order: &names, dependencies: []dingo.Module{catalog}}}
	brand := &brandModule{&namedModule{name: "brand", order: &names, dependencies: []dingo.Module{catalog}}}
	shop := &shopModule{&namedModule{name: "shop", order: &names, dependencies: []dingo.Module{basket, brand}}}

	newInjector(t, shop, brand, basket)

	assert.Equal(t, []string{"catalog", "basket", "brand", "shop"}, names)
}

// The items of a multibinding are in the order in which their modules were
// configured.
func TestModules_MultibindingOrderIsConfigureOrder(t *testing.T) {
	t.Parallel()

	var names []string

	catalog := &catalogModule{&namedModule{name: "catalog", order: &names}}
	basket := &basketModule{&namedModule{name: "basket", order: &names, dependencies: []dingo.Module{catalog}}}
	brand := &brandModule{&namedModule{name: "brand", order: &names}}

	injector := newInjector(t, brand, basket)

	resolved, err := injector.GetAnnotatedInstance(new([]string), "order")
	require.NoError(t, err)

	assert.Equal(t, []string{"brand", "catalog", "basket"}, names)
	assert.Equal(t, names, resolved)
}

// A ModuleFunc is identified by its function value, not by its type, which is
// the same for all of them.
func TestModules_ModuleFuncIsKeyedByItsFuncValue(t *testing.T) {
	t.Parallel()

	t.Run("same value", func(t *testing.T) {
		t.Parallel()

		calls := 0
		module := dingo.ModuleFunc(func(*dingo.Injector) {
			calls++
		})

		newInjector(t, module, module)

		assert.Equal(t, 1, calls)
	})

	t.Run("closures that capture a variable", func(t *testing.T) {
		t.Parallel()

		var names []string

		modules := make([]dingo.Module, 0, 2)

		for _, name := range []string{"shop", "brand"} {
			modules = append(modules, dingo.ModuleFunc(func(*dingo.Injector) {
				names = append(names, name)
			}))
		}

		newInjector(t, modules...)

		assert.Equal(t, []string{"shop", "brand"}, names)
	})

	t.Run("closures that capture nothing", func(t *testing.T) {
		t.Parallel()

		modules := make([]dingo.Module, 0, 2)

		for range 2 {
			modules = append(modules, dingo.ModuleFunc(func(injector *dingo.Injector) {
				injector.BindMulti(new(string)).AnnotatedWith("closure").ToInstance("configured")
			}))
		}

		injector := newInjector(t, modules...)

		resolved, err := injector.GetAnnotatedInstance(new([]string), "closure")
		require.NoError(t, err)
		assert.Equal(t, []string{"configured"}, resolved)
	})

	t.Run("method value evaluated two times", func(t *testing.T) {
		t.Parallel()

		var names []string

		module := &namedModule{name: "basket", order: &names}

		newInjector(t, dingo.ModuleFunc(module.Configure), dingo.ModuleFunc(module.Configure))

		assert.Equal(t, []string{"basket", "basket"}, names)
	})
}

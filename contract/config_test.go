package contract_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configTarget reads one configuration value per field, each by the key
// "config:" plus a dotted name, the way flamingo modules read their config.
type configTarget struct {
	Name  string  `inject:"config:shop.name"`
	Port  int     `inject:"config:shop.port"`
	Ratio float64 `inject:"config:shop.ratio"`
	Debug bool    `inject:"config:shop.debug"`
}

// emptyCollectionsTarget has fields of collection types that nothing, or only
// some of them, is bound for.
type emptyCollectionsTarget struct {
	Names    []string          `inject:"config:catalog.names"`
	Missing  []string          `inject:"config:catalog.missing"`
	Labels   map[string]string `inject:"config:catalog.labels"`
	Greeters []greeter         `inject:""`
}

// Consumers bind configuration as plain values under an annotation. The type
// of the field selects the binding, so one key can hold values of several
// types side by side.
func TestConfig_AnnotationResolvesTheBindingOfTheFieldType(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new(string)).AnnotatedWith("config:shop.name").ToInstance("basket")
		injector.Bind(new(int)).AnnotatedWith("config:shop.port").ToInstance(8080)
		injector.Bind(new(float64)).AnnotatedWith("config:shop.ratio").ToInstance(0.5)
		injector.Bind(true).AnnotatedWith("config:shop.debug").ToInstance(true)
		// Same key as the string above, another type: it must not interfere.
		injector.Bind(new(int)).AnnotatedWith("config:shop.name").ToInstance(1)
	}))

	var target configTarget

	require.NoError(t, injector.RequestInjection(&target))

	assert.Equal(t, configTarget{Name: "basket", Port: 8080, Ratio: 0.5, Debug: true}, target)
}

// A binding made from a value, not from a pointer to a type, is a normal
// binding of that value's type and is found by the same lookups.
func TestConfig_ValueBindingIsAKey(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(true).AnnotatedWith("config:flag").ToInstance(true)
		injector.Bind("").AnnotatedWith("config:word").ToInstance("brand")
	}))

	flag, err := injector.GetAnnotatedInstance(new(bool), "config:flag")
	require.NoError(t, err)
	assert.Equal(t, true, flag)

	word, err := injector.GetAnnotatedInstance(new(string), "config:word")
	require.NoError(t, err)
	assert.Equal(t, "brand", word)
}

// Fields of slice and map type resolve to empty collections when nothing is
// bound for them, so consumers can range over them without a nil check.
func TestLatent_UnboundSliceAndMapFieldsResolveEmpty(t *testing.T) {
	t.Parallel()

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.Bind(new([]string)).AnnotatedWith("config:catalog.names").ToInstance([]string{"shop", "brand"})
	}))

	var target emptyCollectionsTarget

	require.NoError(t, injector.RequestInjection(&target))

	assert.Equal(t, []string{"shop", "brand"}, target.Names)
	assert.Empty(t, target.Missing)
	assert.Empty(t, target.Labels)
	assert.Empty(t, target.Greeters)
}

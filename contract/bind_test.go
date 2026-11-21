package contract_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Binding a type that does not implement the bound interface is a
// programming error and panics at bind time.
func TestBind_ToTypeMismatchPanics(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	assert.Panics(t, func() {
		injector.Bind(new(greeter)).To(counter{})
	})
}

// To takes only the type of the value it is given. The value itself is not
// used, so every resolution builds a new, zero instance of that type.
func TestBind_ToUsesOnlyTheTypeOfTheValue(t *testing.T) {
	t.Parallel()

	given := &helloGreeter{Name: "given"}

	injector := newInjector(t, dingo.ModuleFunc(func(injector *dingo.Injector) {
		injector.BindMulti(new(greeter)).To(given)
	}))

	resolved, err := injector.GetInstance(new([]greeter))
	require.NoError(t, err)

	greeters := resolved.([]greeter)
	require.Len(t, greeters, 1)

	element, ok := greeters[0].(*helloGreeter)
	require.True(t, ok)

	assert.NotSame(t, given, element)
	assert.Empty(t, element.Name)
}

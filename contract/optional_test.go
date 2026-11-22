package contract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// optionalFields marks every dependency optional, with nothing bound.
type optionalFields struct {
	Greeter  greeter   `inject:"optional"`
	Port     int       `inject:"config:shop.port,optional"`
	Name     string    `inject:"config:shop.name,optional"`
	Fallback []greeter `inject:"fallback,optional"`
}

// optionalDependency has a dependency of its own, to show that it is injected.
type optionalDependency struct {
	Greeter greeter `inject:""`
}

// optionalPointers asks for a pointer to a struct, plain and annotated, both
// optional.
type optionalPointers struct {
	Plain     *optionalDependency `inject:"optional"`
	Annotated *optionalDependency `inject:"basket,optional"`
}

// An optional dependency that is not bound stays at its zero value and is no
// error.
func TestOptional_UnboundValuesStayZero(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	var target optionalFields

	require.NoError(t, injector.RequestInjection(&target))

	assert.Nil(t, target.Greeter)
	assert.Zero(t, target.Port)
	assert.Empty(t, target.Name)
	assert.Empty(t, target.Fallback)
}

// An optional pointer to a struct is not left nil, even when it is not bound.
// Each field gets its own new instance, with its dependencies injected.
func TestLatent_OptionalStructPointerGetsAFreshInstance(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.Bind(new(greeter)).To(helloGreeter{})

	var target optionalPointers

	require.NoError(t, injector.RequestInjection(&target))

	require.NotNil(t, target.Plain)
	require.NotNil(t, target.Annotated)
	assert.NotSame(t, target.Plain, target.Annotated)
	assert.Equal(t, "hello", target.Plain.Greeter.Greet())
	assert.Equal(t, "hello", target.Annotated.Greeter.Greet())
}

package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// injectorField receives the injector that resolved it, through the self
// binding every injector makes for itself.
type injectorField struct {
	Injector *dingo.Injector `inject:""`
}

func TestChild_LocalBindingInvisibleToParentAndSiblings(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	child := parent.child(t)
	sibling := parent.child(t)

	child.Bind(new(greeter)).To(helloGreeter{})

	resolved, err := child.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hello", resolved.(greeter).Greet())

	_, err = parent.GetInstance(new(greeter))
	require.Error(t, err)
	assert.ErrorContains(t, err, "can not instantiate interface")

	_, err = sibling.GetInstance(new(greeter))
	require.Error(t, err)
	assert.ErrorContains(t, err, "can not instantiate interface")
}

func TestChild_LocalBindingShadowsParentBinding(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	parent.Bind(new(greeter)).To(helloGreeter{})

	child := parent.child(t)
	child.Bind(new(greeter)).To(loudGreeter{})

	sibling := parent.child(t)

	fromChild, err := child.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "HELLO", fromChild.(greeter).Greet())

	fromParent, err := parent.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hello", fromParent.(greeter).Greet())

	fromSibling, err := sibling.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hello", fromSibling.(greeter).Greet())
}

func TestInjection_InjectorSelfBindingReturnsCurrentInjector(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	child := parent.child(t)

	resolvedInChild, err := child.GetInstance(new(injectorField))
	require.NoError(t, err)
	assert.Same(t, child.Injector, resolvedInChild.(*injectorField).Injector)

	resolvedInParent, err := parent.GetInstance(new(injectorField))
	require.NoError(t, err)
	assert.Same(t, parent.Injector, resolvedInParent.(*injectorField).Injector)
}

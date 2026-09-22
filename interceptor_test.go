package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// exclaimingGreeter is an interceptor with a named, non-embedded field 0.
	// Field 0 must be exported: the engine writes the intercepted value into
	// it with reflect, which panics on an unexported field.
	exclaimingGreeter struct {
		Wrapped greeter
	}

	// whisperingGreeter is a second interceptor, for chain-order assertions.
	whisperingGreeter struct {
		Wrapped greeter
	}
)

func (g *exclaimingGreeter) Greet() string {
	return g.Wrapped.Greet() + "!"
}

func (g *whisperingGreeter) Greet() string {
	return g.Wrapped.Greet() + "..."
}

func TestInterceptor_Field0ReceivesTheBaseValuePositionally(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.Bind(new(greeter)).To(helloGreeter{})
	injector.BindInterceptor(new(greeter), exclaimingGreeter{})

	resolved, err := injector.GetInstance(new(greeter))
	require.NoError(t, err)

	wrapper, ok := resolved.(*exclaimingGreeter)
	require.True(t, ok)
	assert.IsType(t, new(helloGreeter), wrapper.Wrapped)
	assert.Equal(t, "hello!", wrapper.Greet())
}

func TestInterceptor_ReWrapsOnEveryResolutionEvenWithASingletonBase(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: interception runs after the scope
	// cache, so a scoped base is built once while its wrapper is rebuilt per
	// resolution and cannot carry state between them.
	injector := newInjector(t)
	injector.Bind(new(greeter)).To(helloGreeter{}).In(dingo.Singleton)
	injector.BindInterceptor(new(greeter), exclaimingGreeter{})

	first, err := injector.GetInstance(new(greeter))
	require.NoError(t, err)

	second, err := injector.GetInstance(new(greeter))
	require.NoError(t, err)

	firstWrapper, ok := first.(*exclaimingGreeter)
	require.True(t, ok)

	secondWrapper, ok := second.(*exclaimingGreeter)
	require.True(t, ok)

	assert.NotSame(t, firstWrapper, secondWrapper)
	assert.Same(t, firstWrapper.Wrapped, secondWrapper.Wrapped)
}

func TestChild_InterceptorsInheritedFromParentUnconditionally(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: a parent's interceptors always wrap
	// a value resolved through a child, outside the child's own, and a
	// resolution started at the parent never sees the child's.
	parent := newInjector(t)
	parent.Bind(new(greeter)).To(helloGreeter{})
	parent.BindInterceptor(new(greeter), exclaimingGreeter{})

	child := parent.child(t)
	child.BindInterceptor(new(greeter), whisperingGreeter{})

	fromParent, err := parent.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hello!", fromParent.(greeter).Greet())

	fromChild, err := child.GetInstance(new(greeter))
	require.NoError(t, err)
	assert.Equal(t, "hello...!", fromChild.(greeter).Greet())
}

func TestInterceptor_AppliesAcrossChildBoundary(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	parent.Bind(new(greeter)).To(helloGreeter{})
	parent.BindInterceptor(new(greeter), exclaimingGreeter{})

	child := parent.child(t)

	resolved, err := child.GetInstance(new(greeter))
	require.NoError(t, err)

	wrapper, ok := resolved.(*exclaimingGreeter)
	require.True(t, ok)
	assert.Equal(t, "hello!", wrapper.Greet())
}

package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindMulti_DuplicateImplementationsAreAllKept(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: multibinding entries are never
	// deduplicated, unlike singular bindings for one key.
	injector := newInjector(t)
	injector.BindMulti(new(greeter)).To(helloGreeter{})
	injector.BindMulti(new(greeter)).To(helloGreeter{})

	resolved, err := injector.GetInstance(new(greeterSlice))
	require.NoError(t, err)

	assert.Len(t, resolved.(*greeterSlice).All, 2)
}

func TestBindMulti_AnnotationsProduceDisjointSlices(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.BindMulti(new(greeter)).To(helloGreeter{})
	injector.BindMulti(new(greeter)).AnnotatedWith("loud").To(loudGreeter{})

	resolved, err := injector.GetInstance(new(annotatedGreeterSlice))
	require.NoError(t, err)

	slices := resolved.(*annotatedGreeterSlice)
	require.Len(t, slices.Plain, 1)
	require.Len(t, slices.Annotated, 1)
	assert.Equal(t, "hello", slices.Plain[0].Greet())
	assert.Equal(t, "HELLO", slices.Annotated[0].Greet())
}

func TestBindMulti_EmptyResolvesToNonNilEmptySlice(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	resolved, err := injector.GetInstance(new([]greeter))
	require.NoError(t, err)

	assert.NotNil(t, resolved)
	assert.Empty(t, resolved.([]greeter))
}

func TestBindMap_EmptyResolvesToNonNilEmptyMap(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)

	resolved, err := injector.GetInstance(new(map[string]greeter))
	require.NoError(t, err)

	assert.NotNil(t, resolved)
	assert.Empty(t, resolved.(map[string]greeter))
}

func TestBindMap_DuplicateKeySilentlyOverwrites(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: a second BindMap for one key wins
	// without an error, where two singular bindings for one key would fail.
	injector := newInjector(t)
	injector.BindMap(new(greeter), "greeter").To(helloGreeter{})
	injector.BindMap(new(greeter), "greeter").To(loudGreeter{})

	resolved, err := injector.GetInstance(new(greeterMap))
	require.NoError(t, err)

	entries := resolved.(*greeterMap).All
	require.Len(t, entries, 1)
	assert.Equal(t, "HELLO", entries["greeter"].Greet())
}

func TestBindMap_AnnotationsProduceDisjointMaps(t *testing.T) {
	t.Parallel()

	injector := newInjector(t)
	injector.BindMap(new(greeter), "plain").To(helloGreeter{})
	injector.BindMap(new(greeter), "annotated").AnnotatedWith("loud").To(loudGreeter{})

	resolved, err := injector.GetInstance(new(annotatedGreeterMap))
	require.NoError(t, err)

	maps := resolved.(*annotatedGreeterMap)
	assert.Equal(t, []string{"plain"}, keysOf(maps.Plain))
	assert.Equal(t, []string{"annotated"}, keysOf(maps.Annotated))
}

func TestBindMap_ChildOverridesParentKey(t *testing.T) {
	t.Parallel()

	parent := newInjector(t)
	parent.BindMap(new(greeter), "shared").To(helloGreeter{})
	parent.BindMap(new(greeter), "parentOnly").To(helloGreeter{})

	child := parent.child(t)
	child.BindMap(new(greeter), "shared").To(loudGreeter{})

	resolved, err := child.GetInstance(new(greeterMap))
	require.NoError(t, err)

	fromChild := resolved.(*greeterMap).All
	require.Len(t, fromChild, 2)
	assert.Equal(t, "HELLO", fromChild["shared"].Greet())
	assert.Equal(t, "hello", fromChild["parentOnly"].Greet())

	resolved, err = parent.GetInstance(new(greeterMap))
	require.NoError(t, err)

	fromParent := resolved.(*greeterMap).All
	require.Len(t, fromParent, 2)
	assert.Equal(t, "hello", fromParent["shared"].Greet())
}

func TestBindMulti_TargetlessDirectSliceBindingScopesTheMultibinding(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: a direct slice binding with no
	// target falls through to the multibinding entries, which is how the
	// assembled slice gets a scope at all.
	injector := newInjector(t)
	injector.Bind(new([]greeter)).In(dingo.Singleton)
	injector.BindMulti(new(greeter)).To(helloGreeter{})

	first, err := injector.GetInstance(new([]greeter))
	require.NoError(t, err)

	second, err := injector.GetInstance(new([]greeter))
	require.NoError(t, err)

	firstSlice := first.([]greeter)
	secondSlice := second.([]greeter)

	require.Len(t, firstSlice, 1)
	require.Len(t, secondSlice, 1)
	assert.Same(t, &firstSlice[0], &secondSlice[0])

	unscoped := newInjector(t)
	unscoped.BindMulti(new(greeter)).To(helloGreeter{})

	third, err := unscoped.GetInstance(new([]greeter))
	require.NoError(t, err)

	fourth, err := unscoped.GetInstance(new([]greeter))
	require.NoError(t, err)

	assert.NotSame(t, &third.([]greeter)[0], &fourth.([]greeter)[0])
}

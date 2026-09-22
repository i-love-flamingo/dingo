package dingo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBind_DirectSliceBindingWinsOverMultibinding(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: bindings are consulted before
	// slice resolution, so a direct binding with a target hides the
	// multibinding entirely.
	injector := newInjector(t)
	injector.Bind(new([]greeter)).ToInstance([]greeter{loudGreeter{}})
	injector.BindMulti(new(greeter)).To(helloGreeter{})

	resolved, err := injector.GetInstance(new(greeterSlice))
	require.NoError(t, err)

	entries := resolved.(*greeterSlice).All
	require.Len(t, entries, 1)
	assert.Equal(t, "HELLO", entries[0].Greet())
}

func TestBind_DirectMapBindingWinsOverBindMap(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: the map counterpart of the rule
	// above.
	injector := newInjector(t)
	injector.Bind(new(map[string]greeter)).ToInstance(map[string]greeter{"direct": loudGreeter{}})
	injector.BindMap(new(greeter), "viaBindMap").To(helloGreeter{})

	resolved, err := injector.GetInstance(new(greeterMap))
	require.NoError(t, err)

	assert.Equal(t, []string{"direct"}, keysOf(resolved.(*greeterMap).All))
}

func TestBind_DirectProviderTypeBindingWinsOverGeneration(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: bindings are also consulted before
	// the Provider-suffix rule that generates a provider implementation.
	injector := newInjector(t)
	injector.Bind(new(greeter)).To(helloGreeter{})
	injector.Bind(new(greeterProvider)).ToInstance(greeterProvider(func() greeter {
		return loudGreeter{}
	}))

	resolved, err := injector.GetInstance(new(greeterProviderField))
	require.NoError(t, err)
	assert.Equal(t, "HELLO", resolved.(*greeterProviderField).Provide().Greet())

	generated := newInjector(t)
	generated.Bind(new(greeter)).To(helloGreeter{})

	resolved, err = generated.GetInstance(new(greeterProviderField))
	require.NoError(t, err)
	assert.Equal(t, "hello", resolved.(*greeterProviderField).Provide().Greet())
}

func TestGetAnnotatedInstance_MapPrefixIsASpecialLookup(t *testing.T) {
	t.Parallel()

	// Observed behavior, not a guarantee: "map:" is an internal convention
	// on a public method, with a length guard and a precedence rule that are
	// documented nowhere else.

	t.Run("fetches one map binding entry", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)
		injector.BindMap(new(greeter), "hello").To(helloGreeter{})

		resolved, err := injector.GetAnnotatedInstance(new(greeter), "map:hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", resolved.(greeter).Greet())
	})

	t.Run("the four character annotation is not a map lookup", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)
		injector.BindMap(new(greeter), "hello").To(helloGreeter{})

		_, err := injector.GetAnnotatedInstance(new(greeter), "map:")

		require.Error(t, err)
		assert.ErrorContains(t, err, "can not automatically create an annotated injection")
	})

	t.Run("a singular binding with the same annotation wins", func(t *testing.T) {
		t.Parallel()

		injector := newInjector(t)
		injector.BindMap(new(greeter), "hello").To(helloGreeter{})
		injector.Bind(new(greeter)).AnnotatedWith("map:hello").To(loudGreeter{})

		resolved, err := injector.GetAnnotatedInstance(new(greeter), "map:hello")
		require.NoError(t, err)
		assert.Equal(t, "HELLO", resolved.(greeter).Greet())
	})
}

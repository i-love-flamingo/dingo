package dingo_test

import (
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type greeterProvider func() greeter
type greeterErrProvider func() (greeter, error)

// A func() (T, error) provider field returns the value with a nil error on success.
func TestCanErrorProviderSuccess(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*greeter)(nil)).ToProvider(func() greeter { return english{} })

	type holder struct {
		P greeterErrProvider `inject:""`
	}
	got, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	g, perr := got.(*holder).P()
	require.NoError(t, perr)
	assert.Equal(t, "hello", g.Greet())
}

// Provider-typed multibinding: each bound entry becomes a callable provider.
func TestProviderTypedMultibinding(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.BindMulti((*greeter)(nil)).To(english{})
	inj.BindMulti((*greeter)(nil)).To(german{})

	type holder struct {
		Ps []greeterProvider `inject:""`
	}
	got, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	ps := got.(*holder).Ps
	require.Len(t, ps, 2)
	assert.Equal(t, "hello", ps[0]().Greet())
	assert.Equal(t, "hallo", ps[1]().Greet())
}

// Provider-typed map binding.
func TestProviderTypedMapbinding(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.BindMap((*greeter)(nil), "en").To(english{})
	inj.BindMap((*greeter)(nil), "de").To(german{})

	type holder struct {
		Ps map[string]greeterProvider `inject:""`
	}
	got, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	ps := got.(*holder).Ps
	require.Len(t, ps, 2)
	assert.Equal(t, "hello", ps["en"]().Greet())
	assert.Equal(t, "hallo", ps["de"]().Greet())
}

// Provider-typed multibinding whose element can error.
func TestProviderTypedMultibindingCanError(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.BindMulti((*greeter)(nil)).To(english{})

	type holder struct {
		Ps []greeterErrProvider `inject:""`
	}
	got, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	ps := got.(*holder).Ps
	require.Len(t, ps, 1)
	g, perr := ps[0]()
	require.NoError(t, perr)
	assert.Equal(t, "hello", g.Greet())
}

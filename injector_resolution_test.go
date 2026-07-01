package dingo_test

import (
	"errors"
	"testing"

	"flamingo.me/dingo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type greeter interface{ Greet() string }

type english struct{}

func (english) Greet() string { return "hello" }

type german struct{}

func (german) Greet() string { return "hallo" }

func TestBindToConcrete(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*greeter)(nil)).To(english{})
	got, err := inj.GetInstance((*greeter)(nil))
	require.NoError(t, err)
	assert.Equal(t, "hello", got.(greeter).Greet())
}

func TestBindToInstance(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*greeter)(nil)).ToInstance(english{})
	got, err := inj.GetInstance((*greeter)(nil))
	require.NoError(t, err)
	assert.Equal(t, "hello", got.(greeter).Greet())
}

func TestProviderArgsAreInjected(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*string)(nil)).ToInstance("hi")
	inj.Bind((*greeter)(nil)).ToProvider(func(s string) greeter {
		assert.Equal(t, "hi", s)
		return english{}
	})
	got, err := inj.GetInstance((*greeter)(nil))
	require.NoError(t, err)
	assert.Equal(t, "hello", got.(greeter).Greet())
}

func TestAnnotatedBindingsSelectedByAnnotation(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	inj.Bind((*greeter)(nil)).AnnotatedWith("en").To(english{})
	inj.Bind((*greeter)(nil)).AnnotatedWith("de").To(german{})
	en, err := inj.GetAnnotatedInstance((*greeter)(nil), "en")
	require.NoError(t, err)
	assert.Equal(t, "hello", en.(greeter).Greet())
	de, err := inj.GetAnnotatedInstance((*greeter)(nil), "de")
	require.NoError(t, err)
	assert.Equal(t, "hallo", de.(greeter).Greet())
}

func TestUnboundInterfaceErrors(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	_, err = inj.GetInstance((*greeter)(nil))
	assert.Error(t, err)
}

func TestOptionalFieldToleratesMissingBinding(t *testing.T) {
	t.Parallel()
	type target struct {
		Must     string `inject:"must"`
		Optional string `inject:"maybe,optional"`
	}
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	_, err = inj.GetInstance(new(target))
	require.Error(t, err, "missing required value -> error")
	inj.Bind((*string)(nil)).AnnotatedWith("must").ToInstance("x")
	got, err := inj.GetInstance(new(target))
	require.NoError(t, err)
	assert.Equal(t, "x", got.(*target).Must)
	assert.Equal(t, "", got.(*target).Optional)
}

type failingProvider func() (greeter, error)

func TestProviderErrorPropagates(t *testing.T) {
	t.Parallel()
	t.Skip("BUG: ToProvider's error return is silently dropped by Provider.Create — see DINGO-FINDINGS.md#2")
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	boom := errors.New("boom")
	inj.Bind((*greeter)(nil)).ToProvider(func() (greeter, error) { return nil, boom })
	type holder struct {
		P failingProvider `inject:""`
	}
	got, err := inj.GetInstance(new(holder))
	require.NoError(t, err)
	_, perr := got.(*holder).P()
	assert.Error(t, perr, "provider error must surface when called")
}

func TestBindToNonAssignablePanics(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	assert.Panics(t, func() { inj.Bind((*greeter)(nil)).To(123) })
}

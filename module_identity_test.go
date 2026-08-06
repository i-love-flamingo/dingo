package dingo_test

import (
	"testing"

	"flamingo.me/dingo"

	commercecart "flamingo.me/dingo/testdata/moduleidentity/commerce/cart"
	om3cart "flamingo.me/dingo/testdata/moduleidentity/om3/cart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitModules_DistinctPackagesSameTypeName(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	commerceConfigured := 0
	om3Configured := 0
	commerceModule := &commercecart.Module{
		OnConfigure: func() {
			commerceConfigured++
		},
	}
	om3Module := &om3cart.Module{
		OnConfigure: func() {
			om3Configured++
		},
	}

	// Both packages expose a cart.Module; both must be configured, not collapsed.
	require.NoError(t, injector.InitModules(commerceModule, commerceModule, om3Module, om3Module))
	assert.Equal(t, 1, commerceConfigured)
	assert.Equal(t, 1, om3Configured)

	_, err = injector.GetInstance((*commercecart.Service)(nil))
	require.NoError(t, err)

	_, err = injector.GetInstance((*om3cart.Service)(nil))
	require.NoError(t, err)
}

func TestInitModules_DistinctAnonymousModulesSameEmbeddedTypeName(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	commerceModule := &struct{ *commercecart.Module }{
		Module: new(commercecart.Module),
	}
	om3Module := &struct{ *om3cart.Module }{
		Module: new(om3cart.Module),
	}

	require.NoError(t, injector.InitModules(commerceModule, om3Module))

	_, err = injector.GetInstance((*commercecart.Service)(nil))
	require.NoError(t, err)

	_, err = injector.GetInstance((*om3cart.Service)(nil))
	require.NoError(t, err)
}

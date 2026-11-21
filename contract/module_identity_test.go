package contract_test

import (
	"testing"

	"flamingo.me/dingo"

	commercecart "flamingo.me/dingo/contract/testdata/moduleidentity/commerce/cart"
	shopcart "flamingo.me/dingo/contract/testdata/moduleidentity/shop/cart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type moduleWithDependencies struct {
	dependencies []dingo.Module
}

func (*moduleWithDependencies) Configure(*dingo.Injector) {}

func (module *moduleWithDependencies) Depends() []dingo.Module {
	return module.dependencies
}

func TestInitModules_DistinctPackagesSameTypeName(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	commerceConfigured := 0
	shopConfigured := 0
	commerceModule := &commercecart.Module{
		OnConfigure: func() {
			commerceConfigured++
		},
	}
	shopCartModule := &shopcart.Module{
		OnConfigure: func() {
			shopConfigured++
		},
	}

	// Both packages expose a cart.Module; both must be configured, not collapsed.
	require.NoError(t, injector.InitModules(commerceModule, commerceModule, shopCartModule, shopCartModule))
	assert.Equal(t, 1, commerceConfigured)
	assert.Equal(t, 1, shopConfigured)

	_, err = injector.GetInstance((*commercecart.Service)(nil))
	require.NoError(t, err)

	_, err = injector.GetInstance((*shopcart.Service)(nil))
	require.NoError(t, err)
}

func TestInitModules_DistinctPackagesSameTypeNameAsDependencies(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	commerceConfigured := 0
	shopConfigured := 0
	root := &moduleWithDependencies{
		dependencies: []dingo.Module{
			&commercecart.Module{OnConfigure: func() { commerceConfigured++ }},
			&shopcart.Module{OnConfigure: func() { shopConfigured++ }},
		},
	}

	require.NoError(t, injector.InitModules(root))
	assert.Equal(t, 1, commerceConfigured)
	assert.Equal(t, 1, shopConfigured)
}

func TestInitModules_DistinctAnonymousModulesSameEmbeddedTypeName(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	require.NoError(t, err)

	commerceConfigured := 0
	shopConfigured := 0
	commerceModule := &struct{ *commercecart.Module }{
		Module: &commercecart.Module{
			OnConfigure: func() {
				commerceConfigured++
			},
		},
	}
	shopCartModule := &struct{ *shopcart.Module }{
		Module: &shopcart.Module{
			OnConfigure: func() {
				shopConfigured++
			},
		},
	}

	// Both anonymous wrappers embed a same-named cart.Module from a different
	// package. Both promoted Configure methods must run; if the two modules
	// were collapsed into one, only one counter would be incremented.
	require.NoError(t, injector.InitModules(commerceModule, shopCartModule))
	assert.Equal(t, 1, commerceConfigured)
	assert.Equal(t, 1, shopConfigured)

	_, err = injector.GetInstance((*commercecart.Service)(nil))
	require.NoError(t, err)

	_, err = injector.GetInstance((*shopcart.Service)(nil))
	require.NoError(t, err)
}

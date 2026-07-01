package dingo_test

import (
	"testing"

	"flamingo.me/dingo"

	commercecart "flamingo.me/dingo/testdata/moduleidentity/commerce/cart"
	commercecheckout "flamingo.me/dingo/testdata/moduleidentity/commerce/checkout"
	commerceproduct "flamingo.me/dingo/testdata/moduleidentity/commerce/product"
	om3cart "flamingo.me/dingo/testdata/moduleidentity/om3/cart"
	om3checkout "flamingo.me/dingo/testdata/moduleidentity/om3/checkout"
	om3product "flamingo.me/dingo/testdata/moduleidentity/om3/product"
	"flamingo.me/dingo/testdata/multimodule"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSameNamePackagesAreDistinct(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	require.NoError(t, inj.InitModules(new(commercecart.Module), new(om3cart.Module)))
	_, err = inj.GetInstance((*om3cart.Service)(nil))
	assert.NoError(t, err, "binding from the second same-named module must not be dropped")
}

func TestSeveralSameNameCollisions(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	require.NoError(t, inj.InitModules(
		new(commercecart.Module), new(om3cart.Module),
		new(commercecheckout.Module), new(om3checkout.Module),
		new(commerceproduct.Module), new(om3product.Module),
	))
	_, err = inj.GetInstance((*om3checkout.Service)(nil))
	assert.NoError(t, err, "om3 checkout must not be dropped")
	_, err = inj.GetInstance((*om3product.Service)(nil))
	assert.NoError(t, err, "om3 product must not be dropped")
}

func TestSeveralModuleTypesInOnePackage(t *testing.T) {
	t.Parallel()
	inj, err := dingo.NewInjector()
	require.NoError(t, err)
	require.NoError(t, inj.InitModules(
		new(multimodule.Module), new(multimodule.ScopeModule), new(multimodule.SSRModule),
	))
	_, err = inj.GetInstance((*multimodule.MainSvc)(nil))
	assert.NoError(t, err)
	_, err = inj.GetInstance((*multimodule.ScopeSvc)(nil))
	assert.NoError(t, err)
	_, err = inj.GetInstance((*multimodule.SSRSvc)(nil))
	assert.NoError(t, err)
}

func TestModuleFuncIdentity(t *testing.T) {
	t.Parallel()
	shared, separate := 0, 0
	reused := dingo.ModuleFunc(func(*dingo.Injector) { shared++ })

	_, err := dingo.NewInjector(
		reused,
		reused, // same value twice -> runs once
		dingo.ModuleFunc(func(*dingo.Injector) { separate++ }),
		dingo.ModuleFunc(func(*dingo.Injector) { separate++ }), // distinct literal -> runs
	)
	require.NoError(t, err)
	assert.Equal(t, 1, shared, "the same ModuleFunc value must run once")
	assert.Equal(t, 2, separate, "distinct ModuleFunc literals must each run")
}

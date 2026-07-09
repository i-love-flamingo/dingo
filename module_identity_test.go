package dingo_test

import (
	"testing"

	"flamingo.me/dingo"

	commercecart "flamingo.me/dingo/testdata/moduleidentity/commerce/cart"
	om3cart "flamingo.me/dingo/testdata/moduleidentity/om3/cart"
)

func TestInitModules_DistinctPackagesSameTypeName(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	if err != nil {
		t.Fatalf("NewInjector: %v", err)
	}

	// Both packages expose a cart.Module; both must be configured, not collapsed.
	if err := injector.InitModules(new(commercecart.Module), new(om3cart.Module)); err != nil {
		t.Fatalf("InitModules: %v", err)
	}

	if _, err := injector.GetInstance((*commercecart.Service)(nil)); err != nil {
		t.Fatalf("binding from the first same-named module was dropped: %v", err)
	}

	if _, err := injector.GetInstance((*om3cart.Service)(nil)); err != nil {
		t.Fatalf("binding from the second same-named module was dropped: %v", err)
	}
}

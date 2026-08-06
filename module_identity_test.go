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
	if err := injector.InitModules(commerceModule, commerceModule, om3Module, om3Module); err != nil {
		t.Fatalf("InitModules: %v", err)
	}

	if commerceConfigured != 1 {
		t.Fatalf("first same-named module configured %d times, want 1", commerceConfigured)
	}

	if om3Configured != 1 {
		t.Fatalf("second same-named module configured %d times, want 1", om3Configured)
	}

	if _, err := injector.GetInstance((*commercecart.Service)(nil)); err != nil {
		t.Fatalf("binding from the first same-named module was dropped: %v", err)
	}

	if _, err := injector.GetInstance((*om3cart.Service)(nil)); err != nil {
		t.Fatalf("binding from the second same-named module was dropped: %v", err)
	}
}

func TestInitModules_DistinctAnonymousModulesSameEmbeddedTypeName(t *testing.T) {
	t.Parallel()

	injector, err := dingo.NewInjector()
	if err != nil {
		t.Fatalf("NewInjector: %v", err)
	}

	commerceModule := &struct{ *commercecart.Module }{
		Module: new(commercecart.Module),
	}
	om3Module := &struct{ *om3cart.Module }{
		Module: new(om3cart.Module),
	}

	if err := injector.InitModules(commerceModule, om3Module); err != nil {
		t.Fatalf("InitModules: %v", err)
	}

	if _, err := injector.GetInstance((*commercecart.Service)(nil)); err != nil {
		t.Fatalf("binding from the first anonymous module was dropped: %v", err)
	}

	if _, err := injector.GetInstance((*om3cart.Service)(nil)); err != nil {
		t.Fatalf("binding from the second anonymous module was dropped: %v", err)
	}
}

package checkout

import "flamingo.me/dingo"

type Service interface{ Commerce() string }

type serviceImpl struct{}

func (serviceImpl) Commerce() string { return "commerce-checkout" }

type Module struct{}

func (*Module) Configure(injector *dingo.Injector) {
	injector.Bind((*Service)(nil)).To(serviceImpl{})
}

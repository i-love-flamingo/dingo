package checkout

import "flamingo.me/dingo"

type Service interface{ Om3() string }

type serviceImpl struct{}

func (serviceImpl) Om3() string { return "om3-checkout" }

type Module struct{}

func (*Module) Configure(injector *dingo.Injector) {
	injector.Bind((*Service)(nil)).To(serviceImpl{})
}

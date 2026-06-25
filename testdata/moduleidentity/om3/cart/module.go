package cart

import "flamingo.me/dingo"

type Service interface {
	Marker()
}

type serviceImpl struct{}

func (serviceImpl) Marker() {}

type Module struct{}

func (*Module) Configure(injector *dingo.Injector) {
	injector.Bind((*Service)(nil)).To(serviceImpl{})
}

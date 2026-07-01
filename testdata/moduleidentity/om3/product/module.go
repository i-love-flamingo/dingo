package product

import "flamingo.me/dingo"

type Service interface{ Om3() string }

type serviceImpl struct{}

func (serviceImpl) Om3() string { return "om3-product" }

type Module struct{}

func (*Module) Configure(injector *dingo.Injector) {
	injector.Bind((*Service)(nil)).To(serviceImpl{})
}

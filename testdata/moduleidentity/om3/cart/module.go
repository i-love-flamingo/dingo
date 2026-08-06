// Intentionally identical to commerce/cart so Type.String() collides while reflect.Type does not.
package cart

import "flamingo.me/dingo"

type Service interface {
	Marker()
}

type serviceImpl struct{}

func (serviceImpl) Marker() {}

type Module struct {
	OnConfigure func()
}

func (module *Module) Configure(injector *dingo.Injector) {
	if module.OnConfigure != nil {
		module.OnConfigure()
	}

	injector.Bind((*Service)(nil)).To(serviceImpl{})
}

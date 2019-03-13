package paypal

import (
	"flamingo.me/dingo"
	"flamingo.me/dingo/example/application"
)

// Module configures an application to use the paypalCCProcessor for CreditCardProcessing
type Module struct{}

// Configure dependency injection
func (m *Module) Configure(injector *dingo.Injector) {
	injector.Bind(new(application.CreditCardProcessor)).To(new(paypalCCProcessor))
}

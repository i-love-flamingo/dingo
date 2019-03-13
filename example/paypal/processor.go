package paypal

import (
	"log"

	"flamingo.me/dingo/example/application"
)

type paypalCCProcessor struct{}

var _ application.CreditCardProcessor = new(paypalCCProcessor)

func (*paypalCCProcessor) Auth(amount float64) error {
	log.Printf("Paypal: Auth: %v", amount)
	return nil
}

func (*paypalCCProcessor) Capture(amount float64) error {
	log.Printf("Paypal: Capture: %v", amount)
	return nil
}

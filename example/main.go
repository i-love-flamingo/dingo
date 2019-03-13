package main

import (
	"log"

	"flamingo.me/dingo"
	"flamingo.me/dingo/example/application"
	"flamingo.me/dingo/example/paypal"
)

type stdloggerTransactionLog struct {
	prefix string
}

var _ application.TransactionLog = new(stdloggerTransactionLog)

// Log a message with the configure prefix
func (s *stdloggerTransactionLog) Log(id, message string) {
	log.Println(s.prefix, id, message)
}

type defaultModule struct{}

// Configure the dingo injector
func (*defaultModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(application.TransactionLog)).ToInstance(&stdloggerTransactionLog{
		prefix: "example",
	})
}

func main() {
	// create a new injector and load modules
	injector := dingo.NewInjector(
		new(paypal.Module),
		new(defaultModule),
	)

	// instantiate the application service
	service := injector.GetInstance(application.Service{}).(*application.Service)

	// make a transaction
	if err := service.MakeTransaction(99.95, "test transaction"); err != nil {
		log.Fatal(err)
	}
}

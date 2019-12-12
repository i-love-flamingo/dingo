package main

import (
	"log"

	"flamingo.me/dingo"
	"flamingo.me/dingo/miniexample/logger"
)

type stdLogger struct{}

// Log logs a message
func (s *stdLogger) Log(message string) {
	log.Println(message)
}

type loggerModule struct{}

// Configure configures the dingo injector
func (*loggerModule) Configure(injector *dingo.Injector) {
	injector.Bind(new(logger.Logger)).ToInstance(&stdLogger{})
}

func main() {
	// create a new injector
	injector, err := dingo.NewInjector(
		new(loggerModule),
	)
	if err != nil {
		log.Fatal(err)
	}

	// instantiate the log service
	service, err := injector.GetInstance(logger.LogService{})
	if err != nil {
		log.Fatal(err)
	}

	// do a sample log using our service
	service.(*logger.LogService).DoLog("here is an example log")
}

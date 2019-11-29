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
	injector := dingo.NewInjector(
		new(loggerModule),
	)

	// instantiate the log service
	service := injector.GetInstance(logger.LogService{}).(*logger.LogService)

	// do a sample log using our service
	service.DoLog("here is an example log")
}

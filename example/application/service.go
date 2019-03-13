package application

import (
	"fmt"
	"math/rand"
	"strconv"
)

// TransactionLog logs information with a unique id and a message
type TransactionLog interface {
	Log(id, message string)
}

// CreditCardProcessor allows to auth and eventually capture an amount
// float64 is used as an example here
type CreditCardProcessor interface {
	Auth(amount float64) error
	Capture(amount float64) error
}

// Service defines our example application service
type Service struct {
	logger    TransactionLog
	processor CreditCardProcessor
}

// Inject dependencies for our service
func (s *Service) Inject(logger TransactionLog, processor CreditCardProcessor) *Service {
	s.logger = logger
	s.processor = processor
	return s
}

// MakeTransaction tries to authorize and capture an amount, and logs these steps.
func (s *Service) MakeTransaction(amount float64, message string) error {
	id := strconv.Itoa(rand.Int())

	s.logger.Log(id, fmt.Sprintf("Start transaction %q", message))

	s.logger.Log(id, "Try to Auth")
	if err := s.processor.Auth(amount); err != nil {
		s.logger.Log(id, "Auth failed")
		return err
	}

	s.logger.Log(id, "Try to Capture")
	if err := s.processor.Capture(amount); err != nil {
		s.logger.Log(id, "Capture failed")
		return err
	}

	s.logger.Log(id, "Transaction successful")
	return nil
}

package logger

// Logger logs a message
type Logger interface {
	Log(message string)
}

// LogService is a sample service to demonstrate logging
type LogService struct {
	logger Logger
}

// Inject load our dependencies
func (s *LogService) Inject(logger Logger) *LogService {
	s.logger = logger
	return s
}

// DoLog does a sample log
func (s *LogService) DoLog(message string) {
	s.logger.Log(message)
}

package logging

import (
	"context"
	"log"
)

// StandardLogger safely adapts libraries that require the standard library
// logger. The library-controlled message is discarded rather than admitted as
// an unbounded field.
func (l *Logger) StandardLogger(component Component, operation Operation) *log.Logger {
	return log.New(standardWriter{logger: l, component: component, operation: operation}, "", 0)
}

type standardWriter struct {
	logger    *Logger
	component Component
	operation Operation
}

func (w standardWriter) Write(p []byte) (int, error) {
	w.logger.Log(context.Background(), LevelError, EventInternalOperation,
		ComponentField(w.component),
		OperationField(w.operation),
		OutcomeField(OutcomeFailed),
		ErrorCodeField(ErrorInternalFailure),
	)
	return len(p), nil
}

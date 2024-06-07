package common

import (
	"context"

	"github.com/charmbracelet/log"
)

var LogOptions = log.Options{
	ReportTimestamp: true,
	ReportCaller:    true,
	Level:           Options.LogLevel,
}

func RecalculateLogOptions() {
	LogOptions.Level = Options.LogLevel
}

type contextKey string

var ContextLogger = contextKey("log")

func LoggerFromContext(ctx context.Context) *log.Logger {
	logger := ctx.Value(ContextLogger)
	if logger == nil {
		logger = log.Default()
	}
	return logger.(*log.Logger)
}

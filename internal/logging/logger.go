package logging

import (
	"fmt"

	wbflogger "github.com/wb-go/wbf/logger"
)

type EventBookerLogger struct {
	wbflogger.Logger
}

func NewEventBookerLogger(appName, env, engine, level string) (*EventBookerLogger, error) {
	loggerInstance, err := wbflogger.InitLogger(
		parseLoggerEngine(engine),
		appName,
		env,
		wbflogger.WithLevel(parseLogLevel(level)),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	return &EventBookerLogger{
		Logger: loggerInstance,
	}, nil
}

func parseLoggerEngine(value string) wbflogger.Engine {
	switch value {
	case string(wbflogger.ZapEngine):
		return wbflogger.ZapEngine
	case string(wbflogger.ZerologEngine):
		return wbflogger.ZerologEngine
	case string(wbflogger.LogrusEngine):
		return wbflogger.LogrusEngine
	default:
		return wbflogger.SlogEngine
	}
}

func parseLogLevel(value string) wbflogger.Level {
	switch value {
	case "debug":
		return wbflogger.DebugLevel
	case "warn":
		return wbflogger.WarnLevel
	case "error":
		return wbflogger.ErrorLevel
	default:
		return wbflogger.InfoLevel
	}
}

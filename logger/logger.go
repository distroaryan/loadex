package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func InitLogger() {
	logLevel := slog.LevelInfo
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel == "DEBUG" {
		logLevel = slog.LevelDebug
	} else if envLevel == "WARN" {
		logLevel = slog.LevelWarn
	} else if envLevel == "ERROR" {
		logLevel = slog.LevelError
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	Log = slog.New(handler)
	slog.SetDefault(Log)
}

package logging

import (
	"log/slog"
	"os"
	"strings"
)


func Init() (*slog.Logger)  {
	logLevel := getLogLevel()

	opts := &slog.HandlerOptions{
		Level: logLevel,
		AddSource: logLevel == slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	logger := slog.New(handler)

	slog.SetDefault(logger)

	return logger
}


func getLogLevel() slog.Level {
	logLevelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevelStr {
		case "debug":
			return slog.LevelDebug
		case "info":
			return slog.LevelInfo
		case "warn", "warning":
			return slog.LevelWarn
		case "error":
			return slog.LevelError
		default:
			return slog.LevelInfo
	}
}


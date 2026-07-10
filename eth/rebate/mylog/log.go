package mylog

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// 全局 Logger
var Logger zerolog.Logger
var BuilderLogger zerolog.Logger

func init() {
	// 配置 zerolog
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    noColorEnabled() || !stdoutIsTerminal(),
	}
	Logger = zerolog.New(output).With().Timestamp().Logger()
	BuilderLogger = initBuilderLogger()
}

func noColorEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("NO_COLOR")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func initBuilderLogger() zerolog.Logger {
	path := strings.TrimSpace(os.Getenv("BUILDER_REPORT_LOG"))
	if path == "" {
		path = "logs/builder_report.log"
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return zerolog.New(io.Discard)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return zerolog.New(io.Discard)
	}

	return zerolog.New(file).With().
		Timestamp().
		Str("log_type", "builder_report").
		Logger()
}

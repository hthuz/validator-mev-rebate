package mylog

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// 全局 Logger
var Logger zerolog.Logger

func init() {
	// 配置 zerolog
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    noColorEnabled() || !stdoutIsTerminal(),
	}
	Logger = zerolog.New(output).With().Timestamp().Logger()
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

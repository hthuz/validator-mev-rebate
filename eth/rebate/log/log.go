package log

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// 全局 Logger
var Logger zerolog.Logger

func init() {
	// 配置 zerolog
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	Logger = zerolog.New(output).With().Timestamp().Logger()
}

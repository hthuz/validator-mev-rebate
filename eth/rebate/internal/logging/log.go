package logging

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var Logger zerolog.Logger
var BuilderLogger zerolog.Logger

func init() {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    noColorEnabled() || !stdoutIsTerminal(),
	}

	writers := []io.Writer{output}
	if file := openLogFile("rebate.log"); file != nil {
		writers = append(writers, file)
	}

	Logger = zerolog.New(zerolog.MultiLevelWriter(writers...)).With().Timestamp().Logger()
	BuilderLogger = Logger.With().Str("component", "builder").Logger()
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

func openLogFile(name string) *os.File {
	path := filepath.Join(LogDir(), name)

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	return file
}

func LogDir() string {
	if dir := strings.TrimSpace(os.Getenv("REBATE_LOG_DIR")); dir != "" {
		if !filepath.IsAbs(dir) {
			return filepath.Join(projectRoot(), dir)
		}
		return dir
	}
	return filepath.Join(projectRoot(), "logs")
}

func ResolveLogPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if path == "logs" {
		return LogDir()
	}
	if strings.HasPrefix(path, "logs"+string(os.PathSeparator)) || strings.HasPrefix(path, "logs/") {
		return filepath.Join(LogDir(), strings.TrimPrefix(strings.TrimPrefix(path, "logs"+string(os.PathSeparator)), "logs/"))
	}
	return filepath.Join(projectRoot(), path)
}

func projectRoot() string {
	if cwd, err := os.Getwd(); err == nil {
		if root, ok := findModuleRoot(cwd); ok {
			return root
		}
	}

	if _, file, _, ok := runtime.Caller(0); ok {
		if root, ok := findModuleRoot(filepath.Dir(file)); ok {
			return root
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func findModuleRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}

	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(data), "module rebate") {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

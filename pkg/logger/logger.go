package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "INFO"
	}
}

func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

type Logger struct {
	level  Level
	format string // "text" or "json"
	mu     sync.Mutex
	file   *os.File
	path   string
	slog   *slog.Logger
}

var std = &Logger{
	level:  INFO,
	format: strings.ToLower(os.Getenv("BFFX_LOG_FORMAT")),
	path:   os.Getenv("BFFX_LOG_FILE"),
}

// TraceIDExtractor allows external packages (like middleware) to register
// how to pull a trace/request ID from a context.
var TraceIDExtractor func(context.Context) string

func init() {
	if std.path != "" {
		f, err := os.OpenFile(std.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			std.file = f
		}
	}
}

func SetLevel(l Level) {
	std.level = l
}

func SetSlogLogger(l *slog.Logger) {
	std.slog = l
}

func Debug(msg string, args ...any) {
	if std.level <= DEBUG {
		std.log(context.Background(), "DEBUG", msg, args...)
	}
}

func Info(msg string, args ...any) {
	if std.level <= INFO {
		std.log(context.Background(), "INFO", msg, args...)
	}
}

func Warn(msg string, args ...any) {
	if std.level <= WARN {
		std.log(context.Background(), "WARN", msg, args...)
	}
}

func Error(msg string, args ...any) {
	if std.level <= ERROR {
		std.log(context.Background(), "ERROR", msg, args...)
	}
}

func DebugCtx(ctx context.Context, msg string, args ...any) {
	if std.level <= DEBUG {
		std.log(ctx, "DEBUG", msg, args...)
	}
}

func InfoCtx(ctx context.Context, msg string, args ...any) {
	if std.level <= INFO {
		std.log(ctx, "INFO", msg, args...)
	}
}

func WarnCtx(ctx context.Context, msg string, args ...any) {
	if std.level <= WARN {
		std.log(ctx, "WARN", msg, args...)
	}
}

func ErrorCtx(ctx context.Context, msg string, args ...any) {
	if std.level <= ERROR {
		std.log(ctx, "ERROR", msg, args...)
	}
}

func LogMap(level string, data map[string]any) {
	std.logMapCtx(context.Background(), level, data)
}

func LogMapCtx(ctx context.Context, level string, data map[string]any) {
	std.logMapCtx(ctx, level, data)
}

func Fatal(msg string, args ...any) {
	std.log(context.Background(), "FATAL", msg, args...)
	os.Exit(1)
}

func (l *Logger) rotate() {
	if l.file == nil {
		return
	}
	info, err := l.file.Stat()
	if err != nil {
		return
	}
	// Rotate if > 10MB
	if info.Size() < 10*1024*1024 {
		return
	}

	l.file.Close()
	oldPath := l.path + ".old"
	os.Remove(oldPath)
	os.Rename(l.path, oldPath)

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		l.file = f
	}
}

// orderedAccessLogKeys is the preferred emission order for HTTP access logs
// in text mode. Keys not in this list are appended afterwards in
// alphabetical order. Stable ordering keeps `grep`/`awk` cuts predictable
// across processes.
var orderedAccessLogKeys = []string{
	"trace_id",
	"request_id",
	"method",
	"path",
	"status",
	"duration",
	"duration_ms",
	"ip",
	"user_id",
}

func (l *Logger) logMapCtx(ctx context.Context, level string, data map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.rotate()

	if TraceIDExtractor != nil && ctx != nil {
		if tid := TraceIDExtractor(ctx); tid != "" {
			if data == nil {
				data = make(map[string]any)
			}
			data["trace_id"] = tid
		}
	}

	timestamp := time.Now().Format(time.RFC3339)
	if l.slog != nil {
		l.slog.LogAttrs(ctx, slog.LevelInfo, "http", attrsToSlog(data)...)
		return
	}

	if l.format == "json" {
		data["timestamp"] = timestamp
		data["level"] = level
		output, _ := json.Marshal(data)
		fmt.Println(string(output))
		if l.file != nil {
			fmt.Fprintln(l.file, string(output))
		}
		return
	}

	emitted := make(map[string]bool, len(data))
	var sb strings.Builder
	for _, k := range orderedAccessLogKeys {
		if v, ok := data[k]; ok {
			fmt.Fprintf(&sb, "%s=%v ", k, v)
			emitted[k] = true
		}
	}
	extras := make([]string, 0, len(data))
	for k := range data {
		if !emitted[k] {
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)
	for _, k := range extras {
		fmt.Fprintf(&sb, "%s=%v ", k, data[k])
	}

	fmt.Printf("%s [%s] %s\n", timestamp, level, sb.String())
	if l.file != nil {
		fmt.Fprintf(l.file, "%s [%s] %s\n", timestamp, level, sb.String())
	}
}

func (l *Logger) log(ctx context.Context, level, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.rotate()

	timestamp := time.Now().Format(time.RFC3339)
	message := fmt.Sprintf(msg, args...)

	var traceID string
	if TraceIDExtractor != nil && ctx != nil {
		traceID = TraceIDExtractor(ctx)
	}

	// Redaction layer
	sensitiveKeys := []string{"BFFX_JWT_SECRET", "BFFX_ADMIN_SESSION_KEY", "BFFX_APP_SECRET", "BFFX_WORKER_SECRET", "RESEND_API_KEY"}
	for _, key := range sensitiveKeys {
		val := os.Getenv(key)
		if val != "" && len(val) > 4 {
			message = strings.ReplaceAll(message, val, "[REDACTED]")
		}
	}

	if l.slog != nil {
		lvl := slog.LevelInfo
		switch level {
		case "DEBUG": lvl = slog.LevelDebug
		case "WARN": lvl = slog.LevelWarn
		case "ERROR", "FATAL": lvl = slog.LevelError
		}
		l.slog.Log(ctx, lvl, message)
		return
	}

	var output string
	if l.format == "json" {
		entry := map[string]string{
			"timestamp": timestamp,
			"level":     level,
			"message":   message,
		}
		if traceID != "" {
			entry["trace_id"] = traceID
		}
		data, _ := json.Marshal(entry)
		output = string(data)
	} else {
		prefix := ""
		if traceID != "" {
			prefix = fmt.Sprintf("trace_id=%s ", traceID)
		}
		output = fmt.Sprintf("%s [%s] %s%s", timestamp, level, prefix, message)
	}

	fmt.Println(output)
	if l.file != nil {
		fmt.Fprintln(l.file, output)
	}
}

// Redirect stdlib log to our logger for catch-all
func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func Stack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

func attrsToSlog(m map[string]any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(m))
	for k, v := range m {
		attrs = append(attrs, slog.Any(k, v))
	}
	return attrs
}

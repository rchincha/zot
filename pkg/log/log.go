package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultPerms = 0o0600

//nolint:gochecknoglobals
var loggerSetTimeFormat sync.Once

// Event represents a fluent logging interface that maintains zerolog API compatibility.
type Event struct {
	logger    *slog.Logger
	level     slog.Level
	msg       string
	attrs     []slog.Attr
	addCaller bool // whether to add caller information
}

// WithContext represents a logger with additional context that can be chained
type WithContext struct {
	logger    *slog.Logger
	addCaller bool
	attrs     []slog.Attr
}

func (wc *WithContext) Str(key, val string) *WithContext {
	wc.attrs = append(wc.attrs, slog.String(key, val))
	return wc
}

func (wc *WithContext) Int(key string, val int) *WithContext {
	wc.attrs = append(wc.attrs, slog.Int(key, val))
	return wc
}

func (wc *WithContext) Logger() Logger {
	// Create a new logger with the accumulated attributes
	// Convert []slog.Attr to []any for the With method
	args := make([]any, 0, len(wc.attrs)*2)
	for _, attr := range wc.attrs {
		args = append(args, attr.Key, attr.Value.Any())
	}
	newLogger := wc.logger.With(args...)
	return Logger{logger: newLogger, addCaller: wc.addCaller}
}

// Logger wraps slog.Logger to provide zerolog-compatible API.
type Logger struct {
	logger    *slog.Logger
	addCaller bool // whether this logger should add caller info
}

// Event methods for fluent API
func (e *Event) Str(key, val string) *Event {
	e.attrs = append(e.attrs, slog.String(key, val))
	return e
}

func (e *Event) Int(key string, val int) *Event {
	e.attrs = append(e.attrs, slog.Int(key, val))
	return e
}

func (e *Event) Int64(key string, val int64) *Event {
	e.attrs = append(e.attrs, slog.Int64(key, val))
	return e
}

func (e *Event) Float64(key string, val float64) *Event {
	e.attrs = append(e.attrs, slog.Float64(key, val))
	return e
}

func (e *Event) Bool(key string, val bool) *Event {
	e.attrs = append(e.attrs, slog.Bool(key, val))
	return e
}

func (e *Event) Dur(key string, val time.Duration) *Event {
	e.attrs = append(e.attrs, slog.Duration(key, val))
	return e
}

func (e *Event) Time(key string, val time.Time) *Event {
	e.attrs = append(e.attrs, slog.Time(key, val))
	return e
}

func (e *Event) Interface(key string, val interface{}) *Event {
	e.attrs = append(e.attrs, slog.Any(key, val))
	return e
}

func (e *Event) Any(key string, val interface{}) *Event {
	// Alias for Interface to match zerolog API
	return e.Interface(key, val)
}

func (e *Event) Strs(key string, vals []string) *Event {
	e.attrs = append(e.attrs, slog.Any(key, vals))
	return e
}

func (e *Event) IPAddr(key string, ip net.IP) *Event {
	e.attrs = append(e.attrs, slog.String(key, ip.String()))
	return e
}

func (e *Event) RawJSON(key string, b []byte) *Event {
	// For raw JSON, we can store it as a string or try to parse it
	e.attrs = append(e.attrs, slog.String(key, string(b)))
	return e
}

func (e *Event) Uint64(key string, val uint64) *Event {
	e.attrs = append(e.attrs, slog.Uint64(key, val))
	return e
}

func (e *Event) Err(err error) *Event {
	if err != nil {
		e.attrs = append(e.attrs, slog.String("error", err.Error()))
	}
	return e
}

func (e *Event) Msg(msg string) {
	// Get the caller's program counter for accurate source information if needed
	if e.addCaller {
		_, file, line, ok := runtime.Caller(1)
		if ok {
			source := &slog.Source{
				Function: "",
				File:     file,
				Line:     line,
			}
			e.attrs = append(e.attrs, slog.Any(slog.SourceKey, source))
		}
	}
	e.logger.LogAttrs(context.Background(), e.level, msg, e.attrs...)
}

func (e *Event) Msgf(format string, v ...interface{}) {
	// Format the message but keep structured attributes
	formattedMsg := fmt.Sprintf(format, v...)
	// Get the caller's program counter for accurate source information if needed
	if e.addCaller {
		_, file, line, ok := runtime.Caller(1)
		if ok {
			source := &slog.Source{
				Function: "",
				File:     file,
				Line:     line,
			}
			e.attrs = append(e.attrs, slog.Any(slog.SourceKey, source))
		}
	}
	e.logger.LogAttrs(context.Background(), e.level, formattedMsg, e.attrs...)
}

// Logger methods
func (l Logger) Debug() *Event {
	return &Event{logger: l.logger, level: slog.LevelDebug, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
}

func (l Logger) Info() *Event {
	return &Event{logger: l.logger, level: slog.LevelInfo, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
}

func (l Logger) Warn() *Event {
	return &Event{logger: l.logger, level: slog.LevelWarn, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
}

func (l Logger) Error() *Event {
	return &Event{logger: l.logger, level: slog.LevelError, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
}

func (l Logger) Fatal() *Event {
	// slog doesn't have Fatal, but we can use Error level and handle it appropriately
	return &Event{logger: l.logger, level: slog.LevelError, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
}

func (l Logger) Panic() *Event {
	// slog doesn't have Panic, but we can use Error level and handle it appropriately
	return &Event{logger: l.logger, level: slog.LevelError, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
}

func (l Logger) Err(err error) *Event {
	// Start an error-level event with the error
	event := &Event{logger: l.logger, level: slog.LevelError, attrs: make([]slog.Attr, 0), addCaller: l.addCaller}
	if err != nil {
		event.attrs = append(event.attrs, slog.String("error", err.Error()))
	}
	return event
}

func (l Logger) With() *WithContext {
	// Return a WithContext that can be chained with attributes
	return &WithContext{logger: l.logger, addCaller: l.addCaller, attrs: make([]slog.Attr, 0)}
}

func (l Logger) Caller() Logger {
	// Enable caller information for this logger
	return Logger{logger: l.logger, addCaller: true}
}

func (l Logger) Timestamp() Logger {
	// slog handles timestamps automatically
	return l
}

func (l Logger) Logger() Logger {
	// For compatibility with zerolog's chaining
	return l
}

func (l Logger) Println(v ...interface{}) {
	l.Error().Msg("panic recovered") //nolint: check-logs
}

// parseLevelString converts zerolog level strings to slog levels
func parseLevelString(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "fatal", "panic":
		return slog.LevelError // slog doesn't have fatal/panic, use error
	default:
		panic(fmt.Sprintf("unknown level: %s", level))
	}
}

func NewLogger(level, output string) Logger {
	loggerSetTimeFormat.Do(func() {
		// This was used for zerolog time format, slog handles time formatting differently
		// but we keep this for compatibility
	})

	logLevel := parseLevelString(level)

	var writer io.Writer
	if output == "" {
		writer = os.Stdout
	} else {
		file, err := os.OpenFile(output, os.O_APPEND|os.O_WRONLY|os.O_CREATE, defaultPerms)
		if err != nil {
			panic(err)
		}
		writer = file
	}

	// Create JSON handler with options that mimic zerolog behavior
	opts := &slog.HandlerOptions{
		Level: logLevel,
		AddSource: false, // We handle source manually for better compatibility
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize attribute names to match zerolog format
			switch a.Key {
			case slog.TimeKey:
				// Use RFC3339Nano format like zerolog
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String("time", t.Format(time.RFC3339Nano))
				}
			case slog.LevelKey:
				// Convert to lowercase like zerolog
				return slog.String("level", strings.ToLower(a.Value.String()))
			case slog.MessageKey:
				return slog.String("message", a.Value.String())
			case slog.SourceKey:
				// Handle source information to match zerolog's caller format
				if src, ok := a.Value.Any().(*slog.Source); ok {
					return slog.String("caller", fmt.Sprintf("%s:%d", src.File, src.Line))
				}
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(writer, opts)
	
	// Add goroutine hook by wrapping the handler
	wrappedHandler := &goroutineHandler{handler: handler}
	
	logger := slog.New(wrappedHandler)
	
	return Logger{logger: logger, addCaller: true} // Main logger includes caller by default
}

func NewAuditLogger(level, output string) *Logger {
	loggerSetTimeFormat.Do(func() {
		// This was used for zerolog time format, slog handles time formatting differently
		// but we keep this for compatibility
	})

	logLevel := parseLevelString(level)

	var writer io.Writer
	if output == "" {
		writer = os.Stdout
	} else {
		auditFile, err := os.OpenFile(output, os.O_APPEND|os.O_WRONLY|os.O_CREATE, defaultPerms)
		if err != nil {
			panic(err)
		}
		writer = auditFile
	}

	// Create JSON handler with options that mimic zerolog behavior
	// Audit logger doesn't need caller information
	opts := &slog.HandlerOptions{
		Level: logLevel,
		AddSource: false, // Audit logs typically don't need source info
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize attribute names to match zerolog format
			switch a.Key {
			case slog.TimeKey:
				// Use RFC3339Nano format like zerolog
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String("time", t.Format(time.RFC3339Nano))
				}
			case slog.LevelKey:
				// Convert to lowercase like zerolog
				return slog.String("level", strings.ToLower(a.Value.String()))
			case slog.MessageKey:
				return slog.String("message", a.Value.String())
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(writer, opts)
	logger := slog.New(handler)
	
	return &Logger{logger: logger, addCaller: false} // Audit logger doesn't include caller info
}

// GoroutineID adds goroutine-id to logs to help debug concurrency issues.
func GoroutineID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]

	id, err := strconv.Atoi(idField)
	if err != nil {
		return -1
	}

	return id
}

// goroutineHandler wraps an slog.Handler to add goroutine ID to log records
type goroutineHandler struct {
	handler slog.Handler
}

func (h *goroutineHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *goroutineHandler) Handle(ctx context.Context, record slog.Record) error {
	// Add goroutine ID as an attribute
	record.AddAttrs(slog.Int("goroutine", GoroutineID()))
	return h.handler.Handle(ctx, record)
}

func (h *goroutineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &goroutineHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *goroutineHandler) WithGroup(name string) slog.Handler {
	return &goroutineHandler{handler: h.handler.WithGroup(name)}
}

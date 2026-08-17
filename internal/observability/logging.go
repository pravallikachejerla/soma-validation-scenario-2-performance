// Package observability provides structured logging and metrics.
package observability

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Logger writes one JSON object per line to its sink.
type Logger struct {
	mu  sync.Mutex
	out io.Writer
}

// NewLogger returns a logger writing to w.
func NewLogger(w io.Writer) *Logger {
	if w == nil {
		w = os.Stdout
	}
	return &Logger{out: w}
}

// Default is the package-level logger used by application code.
var Default = NewLogger(os.Stdout)

// With returns a child logger with extra context encoded on every line.
func (l *Logger) With(fields map[string]any) *Entry {
	return &Entry{logger: l, fields: fields}
}

// Entry is a pre-bound set of fields.
type Entry struct {
	logger *Logger
	fields map[string]any
}

// Info logs at the info level.
func (e *Entry) Info(msg string, kv ...any) { e.write("info", msg, kv) }

// Warn logs at the warn level.
func (e *Entry) Warn(msg string, kv ...any) { e.write("warn", msg, kv) }

// Error logs at the error level.
func (e *Entry) Error(msg string, kv ...any) { e.write("error", msg, kv) }

// Info is a shortcut on the default logger.
func Info(msg string, kv ...any) { Default.With(nil).Info(msg, kv...) }

// Warn is a shortcut on the default logger.
func Warn(msg string, kv ...any) { Default.With(nil).Warn(msg, kv...) }

// Error is a shortcut on the default logger.
func Error(msg string, kv ...any) { Default.With(nil).Error(msg, kv...) }

func (e *Entry) write(level, msg string, kv []any) {
	rec := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   msg,
	}
	for k, v := range e.fields {
		rec[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		rec[k] = kv[i+1]
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	e.logger.mu.Lock()
	defer e.logger.mu.Unlock()
	_, _ = e.logger.out.Write(append(b, '\n'))
}

// CtxKey is the type used to attach loggers to a context.
type CtxKey struct{}

// WithLogger attaches a logger to the context.
func WithLogger(ctx context.Context, e *Entry) context.Context {
	return context.WithValue(ctx, CtxKey{}, e)
}

// FromContext returns the logger from ctx, or Default.
func FromContext(ctx context.Context) *Entry {
	if v, ok := ctx.Value(CtxKey{}).(*Entry); ok {
		return v
	}
	return Default.With(nil)
}

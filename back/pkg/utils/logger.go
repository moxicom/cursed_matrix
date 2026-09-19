package utils

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	requestIDKey contextKey = "requestID"
	loggerKey    contextKey = "logger"
)

// LoggerOptions keeps this package free of the configuration type, so pkg/
// depends on nothing inside internal/.
type LoggerOptions struct {
	Env     string
	Verbose bool
}

// NewLogger builds the root logger: JSON, with the source position of every call.
func NewLogger(opts LoggerOptions) *slog.Logger {
	level := slog.LevelInfo
	if opts.Verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	return slog.New(&contextHandler{Handler: handler}).With("env", opts.Env)
}

// contextHandler copies the request id out of the context onto every record, so
// a component that logs with InfoContext needs no logger of its own to be
// traceable back to the request. Under an open WithGroup the id nests inside
// that group, which is why nothing here opens one.
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	if id := RequestID(ctx); id != "" {
		record.AddAttrs(slog.String("requestId", id))
	}
	return h.Handler.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}

// ForComponent tags a logger with the part of the system that owns it.
func ForComponent(log *slog.Logger, name string) *slog.Logger {
	return log.With("component", name)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ContextWithLogger carries a request-scoped logger, so code far from the
// composition root can log with the request's fields without taking a logger
// parameter through every call.
func ContextWithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// LoggerFromContext returns the request-scoped logger, falling back to the
// process default for code paths that never passed through the HTTP layer.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}

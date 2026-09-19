package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func captured(t *testing.T, log func(*slog.Logger, context.Context)) map[string]any {
	t.Helper()

	var buffer bytes.Buffer
	handler := slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	logger := slog.New(&contextHandler{Handler: handler})

	log(logger, context.Background())

	var record map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
		t.Fatalf("decode %q: %v", buffer.String(), err)
	}
	return record
}

func TestLoggerRecordCarriesItsContext(t *testing.T) {
	tests := []struct {
		name   string
		log    func(logger *slog.Logger, ctx context.Context)
		want   map[string]string
		absent []string
	}{
		{
			name: "plain call has no request id",
			log: func(logger *slog.Logger, ctx context.Context) {
				logger.InfoContext(ctx, "hello")
			},
			absent: []string{"requestId", "component"},
		},
		{
			name: "request id is copied from the context",
			log: func(logger *slog.Logger, ctx context.Context) {
				logger.InfoContext(WithRequestID(ctx, "req-42"), "hello")
			},
			want: map[string]string{"requestId": "req-42"},
		},
		{
			name: "component tag travels with the logger",
			log: func(logger *slog.Logger, ctx context.Context) {
				ForComponent(logger, "postgres").InfoContext(ctx, "hello")
			},
			want: map[string]string{"component": "postgres"},
		},
		{
			name: "both together",
			log: func(logger *slog.Logger, ctx context.Context) {
				ForComponent(logger, "redis").InfoContext(WithRequestID(ctx, "req-7"), "hello")
			},
			want: map[string]string{"component": "redis", "requestId": "req-7"},
		},
		{
			name: "an open group nests the request id under it",
			log: func(logger *slog.Logger, ctx context.Context) {
				logger.WithGroup("cache").InfoContext(WithRequestID(ctx, "req-9"), "hello")
			},
			absent: []string{"requestId"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record := captured(t, tc.log)

			for key, want := range tc.want {
				if got, _ := record[key].(string); got != want {
					t.Errorf("%s = %v, want %q", key, record[key], want)
				}
			}
			for _, key := range tc.absent {
				if _, present := record[key]; present {
					t.Errorf("%s should not be there: %v", key, record[key])
				}
			}
		})
	}
}

func TestEveryRecordNamesItsSource(t *testing.T) {
	record := captured(t, func(logger *slog.Logger, ctx context.Context) {
		logger.InfoContext(ctx, "hello")
	})

	source, ok := record["source"].(map[string]any)
	if !ok {
		t.Fatalf("no source in %v", record)
	}
	file, _ := source["file"].(string)
	if !strings.HasSuffix(file, "logger_internal_test.go") {
		t.Errorf("source.file = %q, want the calling file", file)
	}
	if line, _ := source["line"].(float64); line == 0 {
		t.Error("source.line is missing")
	}
}

func TestLoggerFromContext(t *testing.T) {
	carried := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)).With("marker", "carried")

	tests := []struct {
		name     string
		ctx      context.Context
		wantSame bool
	}{
		{name: "logger stored in the context", ctx: ContextWithLogger(context.Background(), carried), wantSame: true},
		{name: "nothing stored falls back to the default", ctx: context.Background()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LoggerFromContext(tc.ctx)
			if tc.wantSame && got != carried {
				t.Error("the stored logger was not returned")
			}
			if !tc.wantSame && got != slog.Default() {
				t.Error("the fallback is not the process default")
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{name: "present", ctx: WithRequestID(context.Background(), "abc"), want: "abc"},
		{name: "absent", ctx: context.Background(), want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RequestID(tc.ctx); got != tc.want {
				t.Errorf("RequestID() = %q, want %q", got, tc.want)
			}
		})
	}
}

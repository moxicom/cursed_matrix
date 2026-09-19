package shared_test

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

func TestCodeOf(t *testing.T) {
	plain := errors.New("connection reset")

	tests := []struct {
		name string
		err  error
		want shared.ErrorCode
	}{
		{name: "a domain error reports its own code", err: shared.NewError(shared.CodeSelfLink, nil), want: shared.CodeSelfLink},
		{name: "an unknown error is internal", err: plain, want: shared.CodeInternal},
		{name: "a wrapped domain error keeps its code", err: shared.WrapError(plain, shared.CodeTaskNotFound, nil), want: shared.CodeTaskNotFound},
		{
			name: "a domain error buried under fmt.Errorf is still found",
			err:  fmt.Errorf("loading the board: %w", shared.NewError(shared.CodeUserNotFound, nil)),
			want: shared.CodeUserNotFound,
		},
		{name: "nil is internal", err: nil, want: shared.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shared.CodeOf(tt.err); got != tt.want {
				t.Errorf("CodeOf = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorMessageAndCause(t *testing.T) {
	cause := errors.New("connection reset")

	tests := []struct {
		name      string
		err       *shared.Error
		want      string
		wantCause error
	}{
		{name: "without a cause the code is the message", err: shared.NewError(shared.CodeRateLimited, nil), want: "RATE_LIMITED"},
		{
			name:      "with a cause the message carries both",
			err:       shared.WrapError(cause, shared.CodeInternal, nil),
			want:      "INTERNAL: connection reset",
			wantCause: cause,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
			// The cause must stay reachable: it is what the server log needs
			// and what the client must never see.
			if !errors.Is(tt.err, tt.wantCause) && tt.wantCause != nil {
				t.Errorf("the cause is not unwrappable from %v", tt.err)
			}
		})
	}
}

// TestEnumsRejectUnknownValues covers the boundary where a domain enum becomes
// a native PostgreSQL enum: a value that slipped through would be interpolated
// into a statement the database has no label for.
func TestEnumsRejectUnknownValues(t *testing.T) {
	tests := []struct {
		name    string
		value   driver.Valuer
		wantOK  bool
		wantSQL string
	}{
		{name: "a known quadrant encodes", value: quadrant("IMPORTANT_URGENT"), wantOK: true, wantSQL: "IMPORTANT_URGENT"},
		{name: "an empty quadrant is refused", value: quadrant("")},
		{name: "an invented quadrant is refused", value: quadrant("VERY_URGENT")},
		{name: "a quadrant carrying SQL is refused", value: quadrant("ACTIVE'; DROP TABLE tasks --")},
		{name: "a known status encodes", value: status("COMPLETED"), wantOK: true, wantSQL: "COMPLETED"},
		{name: "a lowercase status is refused", value: status("completed")},
		{name: "a known colour encodes", value: color("CYAN"), wantOK: true, wantSQL: "CYAN"},
		{name: "an invented colour is refused", value: color("BURNT_SIENNA")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.value.Value()
			if tt.wantOK {
				if err != nil {
					t.Fatalf("Value() = %v, want %q", err, tt.wantSQL)
				}
				if got != tt.wantSQL {
					t.Errorf("Value() = %v, want %q", got, tt.wantSQL)
				}
				return
			}
			if err == nil {
				t.Fatalf("Value() accepted %v", got)
			}
		})
	}
}

func TestEnumScan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    shared.Quadrant
		wantErr bool
	}{
		{name: "a string from the driver", src: "IMPORTANT_URGENT", want: shared.QuadrantImportantUrgent},
		{name: "bytes from the driver", src: []byte("IMPORTANT_URGENT"), want: shared.QuadrantImportantUrgent},
		{name: "NULL into a non-pointer is an error", src: nil, wantErr: true},
		{name: "a number is not a quadrant", src: 3, wantErr: true},
		{name: "an unknown label is an error", src: "URGENT_ISH", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got shared.Quadrant
			err := got.Scan(tt.src)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Scan(%v) accepted the value as %q", tt.src, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if got != tt.want {
				t.Errorf("Scan = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRejectsWithAValidationCode(t *testing.T) {
	_, err := shared.ParseQuadrant("NOPE")

	var domainErr *shared.Error
	if !errors.As(err, &domainErr) {
		t.Fatalf("err = %v, want a domain error", err)
	}
	if domainErr.Code != shared.CodeValidationFailed {
		t.Errorf("code = %q, want %q", domainErr.Code, shared.CodeValidationFailed)
	}
	// The field and the offending value are what the client renders.
	if domainErr.Params["field"] != "quadrant" || domainErr.Params["value"] != "NOPE" {
		t.Errorf("params = %v", domainErr.Params)
	}
}

func TestSystemClockIsUTC(t *testing.T) {
	clock := &shared.SystemClock{}
	now := clock.Now()

	if now.Location() != time.UTC {
		t.Errorf("Now() is in %v; every stored instant must be UTC", now.Location())
	}
	if time.Since(now) > time.Minute {
		t.Errorf("Now() = %v, which is not now", now)
	}
}

func quadrant(v string) *shared.Quadrant {
	value := shared.Quadrant(v)
	return &value
}

func status(v string) *shared.TaskStatus {
	value := shared.TaskStatus(v)
	return &value
}

func color(v string) *shared.TaskColor {
	value := shared.TaskColor(v)
	return &value
}

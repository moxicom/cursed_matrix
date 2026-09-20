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

// TestEveryEnumRoundTrips walks all eight of them through the boundary they
// all cross: a known value survives being written and read back, an unknown
// one is refused both ways, and the values listed here are exactly the set
// the package offers.
//
// One table rather than eight near-identical ones: what is being checked is
// that none of them is the odd one out.
func TestEveryEnumRoundTrips(t *testing.T) {
	tests := []struct {
		name     string
		known    []string
		declared int
		unknown  string
		encode   func(string) (driver.Value, error)
		decode   func(any) (string, error)
		parse    func(string) (string, error)
	}{
		{
			name: "quadrant",
			known: []string{
				"IMPORTANT_URGENT", "IMPORTANT_NOT_URGENT",
				"NOT_IMPORTANT_URGENT", "NOT_IMPORTANT_NOT_URGENT",
			},
			declared: len(shared.Quadrants()),
			unknown:  "SOMEWHAT_URGENT",
			encode:   func(v string) (driver.Value, error) { q := shared.Quadrant(v); return q.Value() },
			decode: func(src any) (string, error) {
				var q shared.Quadrant
				if err := q.Scan(src); err != nil {
					return "", err
				}
				return q.String(), nil
			},
			parse: func(v string) (string, error) { q, err := shared.ParseQuadrant(v); return string(q), err },
		},
		{
			name:     "task status",
			known:    []string{"ACTIVE", "COMPLETED"},
			declared: len(shared.TaskStatuses()),
			unknown:  "ABANDONED",
			encode:   func(v string) (driver.Value, error) { s := shared.TaskStatus(v); return s.Value() },
			decode: func(src any) (string, error) {
				var s shared.TaskStatus
				if err := s.Scan(src); err != nil {
					return "", err
				}
				return s.String(), nil
			},
			parse: func(v string) (string, error) { s, err := shared.ParseTaskStatus(v); return string(s), err },
		},
		{
			name:     "completion source",
			known:    []string{"DIRECT", "PARENT_CASCADE"},
			declared: len(shared.CompletionSources()),
			unknown:  "BY_ACCIDENT",
			encode:   func(v string) (driver.Value, error) { c := shared.CompletionSource(v); return c.Value() },
			decode: func(src any) (string, error) {
				var c shared.CompletionSource
				if err := c.Scan(src); err != nil {
					return "", err
				}
				return c.String(), nil
			},
		},
		{
			name:     "language",
			known:    []string{"EN", "RU"},
			declared: len(shared.Languages()),
			unknown:  "FR",
			encode:   func(v string) (driver.Value, error) { l := shared.Language(v); return l.Value() },
			decode: func(src any) (string, error) {
				var l shared.Language
				if err := l.Scan(src); err != nil {
					return "", err
				}
				return l.String(), nil
			},
			parse: func(v string) (string, error) { l, err := shared.ParseLanguage(v); return string(l), err },
		},
		{
			name:     "plan",
			known:    []string{"FREE", "PRO", "SELF_HOSTED"},
			declared: len(shared.Plans()),
			unknown:  "ENTERPRISE",
			encode:   func(v string) (driver.Value, error) { p := shared.Plan(v); return p.Value() },
			decode: func(src any) (string, error) {
				var p shared.Plan
				if err := p.Scan(src); err != nil {
					return "", err
				}
				return p.String(), nil
			},
			parse: func(v string) (string, error) { p, err := shared.ParsePlan(v); return string(p), err },
		},
		{
			name:     "link type",
			known:    []string{"RELATED", "CONNECTED_TO", "BLOCKS", "DEPENDS_ON"},
			declared: len(shared.LinkTypes()),
			unknown:  "SUPERSEDES",
			encode:   func(v string) (driver.Value, error) { l := shared.LinkType(v); return l.Value() },
			decode: func(src any) (string, error) {
				var l shared.LinkType
				if err := l.Scan(src); err != nil {
					return "", err
				}
				return l.String(), nil
			},
		},
		{
			name: "xp source",
			known: []string{
				"TASK_COMPLETED", "SUBTASK_COMPLETED", "ACHIEVEMENT_REWARD",
				"TASK_REOPENED", "ADMIN_ADJUSTMENT", "TASK_DELETED",
			},
			declared: len(shared.XPSources()),
			unknown:  "FOUND_IT",
			encode:   func(v string) (driver.Value, error) { x := shared.XPSource(v); return x.Value() },
			decode: func(src any) (string, error) {
				var x shared.XPSource
				if err := x.Scan(src); err != nil {
					return "", err
				}
				return x.String(), nil
			},
		},
		{
			name: "activity event type",
			known: []string{
				"TASK_CREATED", "TASK_COMPLETED", "SUBTASK_COMPLETED", "TASK_LINKED",
				"LEVEL_UP", "ACHIEVEMENT_UNLOCKED", "STREAK_EXTENDED", "GRAPH_OPENED",
			},
			declared: len(shared.ActivityEventTypes()),
			unknown:  "TASK_POSTPONED",
			encode:   func(v string) (driver.Value, error) { a := shared.ActivityEventType(v); return a.Value() },
			decode: func(src any) (string, error) {
				var a shared.ActivityEventType
				if err := a.Scan(src); err != nil {
					return "", err
				}
				return a.String(), nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A value added to the package and not here would otherwise go
			// through this boundary untested.
			if len(tt.known) != tt.declared {
				t.Fatalf("%d values listed, the package offers %d", len(tt.known), tt.declared)
			}

			for _, value := range tt.known {
				stored, err := tt.encode(value)
				if err != nil {
					t.Fatalf("%q does not encode: %v", value, err)
				}
				if stored != value {
					t.Errorf("%q stored as %v", value, stored)
				}

				// Both shapes a driver may hand back.
				for _, src := range []any{value, []byte(value)} {
					got, err := tt.decode(src)
					if err != nil {
						t.Errorf("%q (%T) does not decode: %v", value, src, err)
					}
					if got != value {
						t.Errorf("%q (%T) decoded as %q", value, src, got)
					}
				}

				if tt.parse != nil {
					parsed, err := tt.parse(value)
					if err != nil || parsed != value {
						t.Errorf("Parse(%q) = %q, %v", value, parsed, err)
					}
				}
			}

			t.Run("an unknown value is refused everywhere", func(t *testing.T) {
				if stored, err := tt.encode(tt.unknown); err == nil {
					t.Errorf("encoded %q as %v", tt.unknown, stored)
				}
				if _, err := tt.decode(tt.unknown); err == nil {
					t.Errorf("decoded %q", tt.unknown)
				}
				if tt.parse != nil {
					if _, err := tt.parse(tt.unknown); err == nil {
						t.Errorf("parsed %q", tt.unknown)
					}
				}
			})
		})
	}
}

func TestLinkDirection(t *testing.T) {
	tests := []struct {
		linkType shared.LinkType
		want     bool
	}{
		{linkType: shared.LinkBlocks, want: true},
		{linkType: shared.LinkDependsOn, want: true},
		{linkType: shared.LinkRelated},
		{linkType: shared.LinkConnectedTo},
	}

	for _, tt := range tests {
		t.Run(string(tt.linkType), func(t *testing.T) {
			if got := tt.linkType.Directed(); got != tt.want {
				t.Errorf("Directed() = %v, want %v", got, tt.want)
			}
		})
	}
}

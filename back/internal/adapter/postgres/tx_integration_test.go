//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/adapter/postgres"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

func newUser(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	suffix := id.String()[:8]
	// No address: the account is identified by its username, and these rows
	// prove the column is genuinely optional.
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, 'x')`,
		id, "tx_"+suffix)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func insertTaskTx(ctx context.Context, t *testing.T, userID uuid.UUID, title string) {
	t.Helper()
	_, err := postgres.ExecInTx(ctx, pool,
		`INSERT INTO tasks (user_id, title, quadrant, position)
		 VALUES ($1, $2, 'IMPORTANT_URGENT', 1024)`, userID, title)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}
}

var errUseCaseFailed = errors.New("the use case failed")

func TestTransactionLifecycle(t *testing.T) {
	tests := []struct {
		name      string
		body      func(ctx context.Context, t *testing.T, manager *postgres.TxManager, userID uuid.UUID) error
		wantErr   error
		wantPanic bool
		wantRows  int
	}{
		{
			name: "a successful use case commits",
			body: func(ctx context.Context, t *testing.T, _ *postgres.TxManager, userID uuid.UUID) error {
				insertTaskTx(ctx, t, userID, "committed")
				return nil
			},
			wantRows: 1,
		},
		{
			name: "a returned error rolls everything back",
			body: func(ctx context.Context, t *testing.T, _ *postgres.TxManager, userID uuid.UUID) error {
				insertTaskTx(ctx, t, userID, "doomed")
				return errUseCaseFailed
			},
			wantErr:  errUseCaseFailed,
			wantRows: 0,
		},
		{
			name: "a panic rolls back and keeps panicking",
			body: func(ctx context.Context, t *testing.T, _ *postgres.TxManager, userID uuid.UUID) error {
				insertTaskTx(ctx, t, userID, "panicking")
				panic("boom")
			},
			wantPanic: true,
			wantRows:  0,
		},
		{
			name: "a nested call joins the same transaction",
			body: func(ctx context.Context, t *testing.T, manager *postgres.TxManager, userID uuid.UUID) error {
				insertTaskTx(ctx, t, userID, "outer")
				return manager.Do(ctx, func(ctx context.Context) error {
					insertTaskTx(ctx, t, userID, "inner")
					return errUseCaseFailed
				})
			},
			wantErr:  errUseCaseFailed,
			wantRows: 0,
		},
		{
			name: "a nested call that succeeds commits both writes",
			body: func(ctx context.Context, t *testing.T, manager *postgres.TxManager, userID uuid.UUID) error {
				insertTaskTx(ctx, t, userID, "outer")
				return manager.Do(ctx, func(ctx context.Context) error {
					insertTaskTx(ctx, t, userID, "inner")
					return nil
				})
			},
			wantRows: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			manager := postgres.NewTxManager(pool, testLogger())
			userID := newUser(t)

			var err error
			run := func() {
				if tc.wantPanic {
					defer func() {
						if recover() == nil {
							t.Error("the panic did not propagate out of Do")
						}
					}()
				}
				err = manager.Do(ctx, func(ctx context.Context) error {
					return tc.body(ctx, t, manager, userID)
				})
			}
			run()

			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Do returned %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && !tc.wantPanic && err != nil {
				t.Fatalf("Do: %v", err)
			}
			if got := countTasks(t, userID); got != tc.wantRows {
				t.Errorf("the user has %d tasks, want %d", got, tc.wantRows)
			}
		})
	}
}

func TestRepositoryReadsSeeTheOpenTransaction(t *testing.T) {
	ctx := context.Background()
	manager := postgres.NewTxManager(pool, testLogger())
	repo := postgres.NewTaskRepository(pool)
	userID := newUser(t)

	err := manager.Do(ctx, func(ctx context.Context) error {
		insertTaskTx(ctx, t, userID, "visible inside")

		tests := []struct {
			name string
			ctx  context.Context
			want int
		}{
			{name: "inside the transaction", ctx: ctx, want: 1},
			{name: "outside the transaction", ctx: context.Background(), want: 0},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				tasks, err := repo.ListBoard(tc.ctx, task.DefaultFilter(userID))
				if err != nil {
					t.Fatalf("ListBoard: %v", err)
				}
				if len(tasks) != tc.want {
					t.Errorf("saw %d tasks, want %d", len(tasks), tc.want)
				}
			})
		}
		return errUseCaseFailed
	})
	if !errors.Is(err, errUseCaseFailed) {
		t.Fatalf("Do returned %v", err)
	}
	if got := countTasks(t, userID); got != 0 {
		t.Errorf("after rollback the user has %d tasks, want 0", got)
	}
}

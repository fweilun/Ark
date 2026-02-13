// README: Concurrency tests for order state transitions (run with -race).
package order

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"

    "ark/internal/types"
)

func TestConcurrentAcceptVsCancel(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    svc := NewService(store, nil)

    orderID, err := svc.Create(ctx, CreateCommand{
        PassengerID: "p_accept_cancel",
        Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
        Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
        RideType:    "economy",
    })
    if err != nil {
        t.Fatalf("create order: %v", err)
    }

    var wg sync.WaitGroup
    errs := make(chan error, 2)

    wg.Add(1)
    go func() {
        defer wg.Done()
        errs <- svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d1"})
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        errs <- svc.Cancel(ctx, CancelCommand{OrderID: orderID, ActorType: "passenger", Reason: "user_cancel"})
    }()

    wg.Wait()
    close(errs)

    success := 0
    for err := range errs {
        if err == nil {
            success++
            continue
        }
        if err != ErrConflict && err != ErrInvalidState {
            t.Fatalf("unexpected error: %v", err)
        }
    }
    if success < 1 || success > 2 {
        t.Fatalf("expected 1 or 2 successes, got %d", success)
    }

    o, err := svc.Get(ctx, orderID)
    if err != nil {
        t.Fatalf("get order: %v", err)
    }
    if success == 2 && o.Status != StatusCancelled {
        t.Fatalf("expected cancelled after accept+cancel, got %s", o.Status)
    }
    if success == 1 && o.Status != StatusApproaching && o.Status != StatusCancelled {
        t.Fatalf("unexpected final status: %s", o.Status)
    }
}

func TestConcurrentAcceptSameOrder(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    svc := NewService(store, nil)

    orderID, err := svc.Create(ctx, CreateCommand{
        PassengerID: "p_multi_accept",
        Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
        Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
        RideType:    "economy",
    })
    if err != nil {
        t.Fatalf("create order: %v", err)
    }

    const attempts = 8
    var wg sync.WaitGroup
    errs := make(chan error, attempts)

    for i := 0; i < attempts; i++ {
        driverID := types.ID(fmt.Sprintf("d%d", i))
        wg.Add(1)
        go func(did types.ID) {
            defer wg.Done()
            errs <- svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: did})
        }(driverID)
    }

    wg.Wait()
    close(errs)

    success := 0
    for err := range errs {
        if err == nil {
            success++
            continue
        }
        if err != ErrConflict && err != ErrInvalidState {
            t.Fatalf("unexpected error: %v", err)
        }
    }
    if success != 1 {
        t.Fatalf("expected exactly 1 success, got %d", success)
    }

    o, err := svc.Get(ctx, orderID)
    if err != nil {
        t.Fatalf("get order: %v", err)
    }
    if o.Status != StatusApproaching {
        t.Fatalf("unexpected final status: %s", o.Status)
    }
    if o.DriverID == nil || *o.DriverID == "" {
        t.Fatalf("expected driver_id to be set")
    }
}

func setupTestStore(t *testing.T) *Store {
    t.Helper()

    dsn := os.Getenv("ARK_TEST_DSN")
    if dsn == "" {
        t.Skip("ARK_TEST_DSN not set; skipping DB-backed race tests")
    }

    ctx := context.Background()
    db, err := pgxpool.New(ctx, dsn)
    if err != nil {
        t.Fatalf("connect db: %v", err)
    }
    t.Cleanup(func() { db.Close() })

    if err := applyMigration(ctx, db); err != nil {
        t.Fatalf("apply migration: %v", err)
    }

    if _, err := db.Exec(ctx, "TRUNCATE TABLE order_state_events, orders"); err != nil {
        t.Fatalf("truncate tables: %v", err)
    }

    return NewStore(db)
}

func applyMigration(ctx context.Context, db *pgxpool.Pool) error {
    root, err := repoRoot()
    if err != nil {
        return err
    }
    path := filepath.Join(root, "migrations", "0001_init.sql")
    content, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    cleaned := stripSQLComments(string(content))
    for _, stmt := range splitSQL(cleaned) {
        if _, err := db.Exec(ctx, stmt); err != nil {
            return err
        }
    }
    return nil
}

func repoRoot() (string, error) {
    dir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    for i := 0; i < 6; i++ {
        if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            break
        }
        dir = parent
    }
    return "", os.ErrNotExist
}

func stripSQLComments(input string) string {
    var b strings.Builder
    scanner := bufio.NewScanner(strings.NewReader(input))
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "--") {
            continue
        }
        b.WriteString(scanner.Text())
        b.WriteString("\n")
    }
    return b.String()
}

func splitSQL(input string) []string {
    parts := strings.Split(input, ";")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        stmt := strings.TrimSpace(p)
        if stmt == "" {
            continue
        }
        out = append(out, stmt)
    }
    return out
}

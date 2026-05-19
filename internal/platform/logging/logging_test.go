package logging_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

func TestFromContext_FallsBackToDefaultWhenUnset(t *testing.T) {
	t.Parallel()
	if logging.FromContext(context.Background()) == nil {
		t.Fatal("FromContext must never return nil")
	}
	//nolint:staticcheck // explicitly testing the nil-context guard
	if logging.FromContext(nil) == nil {
		t.Fatal("FromContext(nil) must never return nil")
	}
}

func TestWithLogger_RoundTrips(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	ctx := logging.WithLogger(context.Background(), cap.Logger())

	logging.FromContext(ctx).Info("hello", slog.String("k", "v"))

	rec, ok := cap.FindByMessage("hello")
	if !ok {
		t.Fatal("expected the record to be captured")
	}
	if rec.Attrs["k"] != "v" {
		t.Fatalf("attr not captured: %#v", rec.Attrs)
	}
	if rec.Level != slog.LevelInfo {
		t.Fatalf("level mismatch: %v", rec.Level)
	}
}

func TestWithLogger_NilLoggerIsNoOp(t *testing.T) {
	t.Parallel()
	parent := context.Background()
	if logging.WithLogger(parent, nil) != parent {
		t.Fatal("WithLogger with nil logger must return the parent unchanged")
	}
}

func TestWith_AddsAttributesToContextLogger(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	ctx := logging.WithLogger(context.Background(), cap.Logger())
	ctx = logging.With(ctx, slog.String("request_id", "abc-123"))

	logging.FromContext(ctx).Warn("tagged")

	rec, ok := cap.FindByMessage("tagged")
	if !ok {
		t.Fatal("record not captured")
	}
	if rec.Attrs["request_id"] != "abc-123" {
		t.Fatalf("inherited attr missing: %#v", rec.Attrs)
	}
}

func TestCapture_LevelFilter(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelWarn)
	l := cap.Logger()
	l.Debug("dropped")
	l.Info("dropped")
	l.Warn("kept")
	l.Error("kept2")

	if len(cap.Records()) != 2 {
		t.Fatalf("expected 2 records above warn, got %d", len(cap.Records()))
	}
}

func TestCapture_ResetAndFindAll(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	l := cap.Logger()
	l.Info("dup")
	l.Info("dup")
	if got := cap.FindAllByMessage("dup"); len(got) != 2 {
		t.Fatalf("FindAllByMessage: want 2, got %d", len(got))
	}
	cap.Reset()
	if len(cap.Records()) != 0 {
		t.Fatal("Reset did not clear records")
	}
}

func TestCapture_IsGoroutineSafe(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	l := cap.Logger()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Info("concurrent")
		}()
	}
	wg.Wait()
	if len(cap.FindAllByMessage("concurrent")) != 50 {
		t.Fatalf("race lost records: got %d", len(cap.FindAllByMessage("concurrent")))
	}
}

func TestCapture_FlattensGroups(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	l := cap.Logger().WithGroup("outer")
	l.Info("grouped", slog.String("inner", "x"))
	rec, ok := cap.FindByMessage("grouped")
	if !ok {
		t.Fatal("record not captured")
	}
	if rec.Attrs["outer.inner"] != "x" {
		t.Fatalf("group not flattened: %#v", rec.Attrs)
	}
}

func TestUserIDAttr_ZeroIsEmptyString(t *testing.T) {
	t.Parallel()
	a := logging.UserID(shared.ID{})
	if a.Value.String() != "" {
		t.Fatalf("zero ID should render as empty string, got %q", a.Value.String())
	}
	id := shared.NewID()
	if logging.UserID(id).Value.String() != id.String() {
		t.Fatal("non-zero ID must render its string form")
	}
}

func TestErrAttr_HandlesNil(t *testing.T) {
	t.Parallel()
	if logging.Err(nil).Key != logging.AttrError {
		t.Fatal("Err must use the canonical key even for nil")
	}
}

package bridge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPairLimiterBoundsActivePairs(t *testing.T) {
	limiter, err := NewPairLimiter(2)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := limiter.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if limiter.Active() != 2 || limiter.Capacity() != 2 {
		t.Fatalf("unexpected limiter state active=%d capacity=%d", limiter.Active(), limiter.Capacity())
	}

	blockedCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := limiter.Acquire(blockedCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline while limiter full, got %v", err)
	}

	limiter.Release()
	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	limiter.Release()
	limiter.Release()
	if limiter.Active() != 0 {
		t.Fatalf("expected empty limiter, got %d", limiter.Active())
	}
}

func TestPairLimiterRejectsInvalidCapacity(t *testing.T) {
	if _, err := NewPairLimiter(0); err == nil {
		t.Fatal("expected zero capacity rejection")
	}
}

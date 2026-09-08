package bridge

import (
	"context"
	"fmt"
)

// PairLimiter bounds the number of active stream/packet pairs across tunnels.
// A slot is reserved before pair acquisition and released when the pair closes
// or acquisition fails. This prevents connection storms from growing active
// data-plane resource use without a configured upper bound.
type PairLimiter struct {
	slots chan struct{}
}

func NewPairLimiter(maxActive int) (*PairLimiter, error) {
	if maxActive < 1 {
		return nil, fmt.Errorf("max active pairs must be positive")
	}
	return &PairLimiter{slots: make(chan struct{}, maxActive)}, nil
}

func (l *PairLimiter) Acquire(ctx context.Context) error {
	if l == nil {
		return nil
	}
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *PairLimiter) Release() {
	if l == nil {
		return
	}
	select {
	case <-l.slots:
	default:
		panic("bridge PairLimiter released without an acquired slot")
	}
}

func (l *PairLimiter) Active() int {
	if l == nil {
		return 0
	}
	return len(l.slots)
}

func (l *PairLimiter) Capacity() int {
	if l == nil {
		return 0
	}
	return cap(l.slots)
}

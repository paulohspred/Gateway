//go:build !linux

package canbridge

import (
	"context"
	"fmt"
	"log/slog"
)

type Config struct {
	ID            string
	Interface     string
	Socket        string
	EnableFD      bool
	ReceiveOwn    bool
	AllowTransmit bool
}

type Hooks struct {
	OnOpen  func(sessionID string)
	OnFrame func(sessionID, direction string, n uint64)
	OnClose func(sessionID string, err error)
}

func Run(context.Context, Config, *slog.Logger, Hooks) error {
	return fmt.Errorf("SocketCAN is supported only on Linux")
}

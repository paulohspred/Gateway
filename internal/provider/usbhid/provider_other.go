//go:build !linux

package usbhid

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

var ErrWriteDisabled = errors.New("USB HID write disabled by configuration")

func Run(_ context.Context, cfg Config, _ *slog.Logger, _ Hooks) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	return fmt.Errorf("USB HID hidraw provider is only supported on Linux")
}

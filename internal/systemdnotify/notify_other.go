//go:build !linux

package systemdnotify

import "context"

type Notifier struct{}

func FromEnv() *Notifier                         { return &Notifier{} }
func (n *Notifier) Enabled() bool                { return false }
func (n *Notifier) WatchdogEnabled() bool        { return false }
func (n *Notifier) Ready() error                 { return nil }
func (n *Notifier) Stopping() error              { return nil }
func (n *Notifier) Notify(string) error          { return nil }
func (n *Notifier) StartWatchdog(context.Context, func(error)) {}

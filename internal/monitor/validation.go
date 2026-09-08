package monitor

import (
	"errors"
	"fmt"
	"strings"
)

func (a Alarm) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return errors.New("alarm id is required")
	}
	if strings.TrimSpace(a.GeneratorID) == "" {
		return errors.New("alarm generator id is required")
	}
	if strings.TrimSpace(a.Code) == "" {
		return errors.New("alarm code is required")
	}
	if !IsValidAlarmSeverity(a.Severity) {
		return fmt.Errorf("alarm %q has invalid severity %q", a.Code, a.Severity)
	}
	if strings.TrimSpace(a.Message) == "" {
		return fmt.Errorf("alarm %q message is required", a.Code)
	}
	if a.RaisedAt.IsZero() {
		return fmt.Errorf("alarm %q raisedAt is required", a.Code)
	}
	if a.Active && a.ClearedAt != nil {
		return fmt.Errorf("active alarm %q must not contain clearedAt", a.Code)
	}
	if a.ClearedAt != nil {
		if a.ClearedAt.IsZero() {
			return fmt.Errorf("alarm %q clearedAt must not be zero", a.Code)
		}
		if a.ClearedAt.Before(a.RaisedAt) {
			return fmt.Errorf("alarm %q clearedAt precedes raisedAt", a.Code)
		}
	}
	return nil
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("event id is required")
	}
	if strings.TrimSpace(e.GeneratorID) == "" {
		return errors.New("event generator id is required")
	}
	if strings.TrimSpace(e.Type) == "" {
		return errors.New("event type is required")
	}
	if strings.ContainsAny(e.Type, " \t\r\n/") {
		return fmt.Errorf("event type %q is invalid", e.Type)
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Errorf("event %q message is required", e.Type)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("event %q occurredAt is required", e.Type)
	}
	return nil
}

func (h ProviderHealth) Validate() error {
	switch h.Status {
	case ProviderHealthy, ProviderDegraded, ProviderUnavailable:
	default:
		return fmt.Errorf("invalid provider health status %q", h.Status)
	}
	if h.CheckedAt.IsZero() {
		return errors.New("provider health checkedAt is required")
	}
	return nil
}

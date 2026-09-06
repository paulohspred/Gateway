package rapid

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
)

type stubRawReader struct {
	current    []ChannelData
	events     []RawEvent
	currentErr error
	eventsErr  error
	healthErr  error
}

func (r *stubRawReader) ReadCurrent(ctx context.Context, _ []int) ([]ChannelData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.currentErr != nil {
		return nil, r.currentErr
	}
	return append([]ChannelData(nil), r.current...), nil
}

func (r *stubRawReader) ReadRecentEvents(ctx context.Context, _ EventQuery) ([]RawEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.eventsErr != nil {
		return nil, r.eventsErr
	}
	return append([]RawEvent(nil), r.events...), nil
}

func (r *stubRawReader) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.healthErr
}

func TestSemanticReaderMapsAlarmAndHistoricalTransitions(t *testing.T) {
	reader := &stubRawReader{
		current: []ChannelData{{ChannelNumber: 115, Value: 1, Status: 1, ObservedAt: testNow}},
		events: []RawEvent{
			{
				ID:             "rapid-42",
				ChannelNumber:  115,
				PreviousValue:  0,
				PreviousStatus: 1,
				Value:          1,
				Status:         1,
				OccurredAt:     testNow.Add(-30 * time.Second),
			},
			{
				ID:             "rapid-43",
				ChannelNumber:  116,
				PreviousValue:  0,
				PreviousStatus: 1,
				Value:          1,
				Status:         1,
				OccurredAt:     testNow.Add(-10 * time.Second),
			},
		},
	}
	semantic := newTestSemanticReader(t, reader)

	alarms, err := semantic.ReadAlarms(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 1 {
		t.Fatalf("expected one active alarm, got %#v", alarms)
	}
	if alarms[0].Code != "LOW_OIL_PRESSURE" || !alarms[0].RaisedAt.Equal(testNow.Add(-30*time.Second)) {
		t.Fatalf("unexpected alarm: %#v", alarms[0])
	}

	events, err := semantic.ReadEvents(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected alarm transition plus explicit event, got %#v", events)
	}
	if events[0].Type != "controller.test_event" || events[1].Type != "alarm.raised" {
		t.Fatalf("unexpected normalized events: %#v", events)
	}
}

func TestSemanticReaderOmitsUndefinedAlarmChannel(t *testing.T) {
	reader := &stubRawReader{current: []ChannelData{{ChannelNumber: 115, Value: 1, Status: 0, ObservedAt: testNow}}}
	semantic := newTestSemanticReader(t, reader)
	alarms, err := semantic.ReadAlarms(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 0 {
		t.Fatalf("undefined Rapid alarm channel must not invent an alarm: %#v", alarms)
	}
}

func TestSemanticReaderUsesDetectionTimeWhenHistoryUnavailable(t *testing.T) {
	reader := &stubRawReader{
		current:   []ChannelData{{ChannelNumber: 115, Value: 1, Status: 1, ObservedAt: testNow}},
		eventsErr: errors.New("history unavailable"),
	}
	semantic := newTestSemanticReader(t, reader)
	alarms, err := semantic.ReadAlarms(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 1 || !alarms[0].RaisedAt.Equal(testNow) {
		t.Fatalf("expected explicit detection-time fallback, got %#v", alarms)
	}
}

func TestConditionSupportsBitfieldsAndThresholds(t *testing.T) {
	bit := uint(3)
	condition := Condition{Kind: ConditionBitSet, Bit: &bit}
	matched, err := condition.Matches(8)
	if err != nil || !matched {
		t.Fatalf("expected bit 3 to match 8: matched=%v err=%v", matched, err)
	}
	matched, err = condition.Matches(4)
	if err != nil || matched {
		t.Fatalf("expected bit 3 not to match 4: matched=%v err=%v", matched, err)
	}

	threshold := 90.0
	condition = Condition{Kind: ConditionGTE, Value: &threshold}
	matched, err = condition.Matches(90)
	if err != nil || !matched {
		t.Fatalf("expected gte threshold to match boundary: matched=%v err=%v", matched, err)
	}
	if _, err := (Condition{Kind: ConditionBitSet, Bit: &bit}).Matches(8.5); err == nil {
		t.Fatal("bit_set must reject non-integer raw values")
	}
}

func TestSemanticReaderPropagatesHealth(t *testing.T) {
	reader := &stubRawReader{healthErr: errors.New("down")}
	semantic := newTestSemanticReader(t, reader)
	if err := semantic.Health(context.Background()); err == nil {
		t.Fatal("expected raw health failure to propagate")
	}
}

func newTestSemanticReader(t *testing.T, raw RawReader) *SemanticReader {
	t.Helper()
	bundle := loadTestProfile(t)
	binding, err := LoadBinding(testBindingPath(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	configs := []GeneratorConfig{{
		Generator: monitor.Generator{
			ID:     "gen-rapid-test",
			Name:   "Rapid Test Generator",
			SiteID: "site-test",
			Controller: monitor.ControllerRef{
				Manufacturer: bundle.Manifest.Manufacturer,
				Model:        bundle.Manifest.Model,
			},
		},
		Profile: bundle,
		Binding: binding,
	}}
	reader, err := NewSemanticReader(raw, configs, SemanticOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func testBindingPath() string {
	return "../../../controllers/rc-simulator/reference-controller/rapid/channels.json"
}

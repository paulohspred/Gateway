import { describe, expect, it } from "vitest";
import type { Telemetry } from "../api/schemas";
import { explicitBreakerState, explicitSourceState } from "./electricalState";

const observedAt = "2026-09-07T11:30:00Z";
function telemetry(metrics: Telemetry["metrics"]): Telemetry { return { generatorId: "g1", capturedAt: observedAt, communication: "online", metrics }; }
function metric(value: number | string | boolean, quality: "good" | "stale" | "offline" | "bad" | "unknown" = "good") { return { value, quality, observedAt }; }

describe("electrical explicit state contract", () => {
  it("does not infer source state from voltage, rpm or power", () => {
    const t = telemetry({ "mains.voltage_l1": metric(230), "engine.rpm": metric(1800), "generator.power_kw": metric(120) });
    expect(explicitSourceState(t, ["mains.state"])).toBeNull();
    expect(explicitSourceState(t, ["generator.status", "engine.state"])).toBeNull();
  });

  it("uses explicit source states only when quality is good", () => {
    const t = telemetry({ "mains.state": metric("online"), "generator.status": metric("stopped"), "engine.state": metric("running", "stale") });
    expect(explicitSourceState(t, ["mains.state"])).toBe(true);
    expect(explicitSourceState(t, ["generator.status", "engine.state"])).toBe(false);
  });

  it("requires an explicit good boolean for breaker state", () => {
    expect(explicitBreakerState(telemetry({ "breaker.gcb": metric(true) }), "breaker.gcb")).toBe(true);
    expect(explicitBreakerState(telemetry({ "breaker.gcb": metric(false) }), "breaker.gcb")).toBe(false);
    expect(explicitBreakerState(telemetry({ "breaker.gcb": metric(true, "stale") }), "breaker.gcb")).toBeNull();
    expect(explicitBreakerState(telemetry({ "breaker.gcb": metric("closed") }), "breaker.gcb")).toBeNull();
  });
});

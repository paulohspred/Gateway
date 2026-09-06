import { describe, expect, it } from "vitest";
import { telemetrySchema } from "./schemas";

describe("telemetry schema", () => {
  it("aceita valores number/text/bool com quality", () => {
    const parsed = telemetrySchema.parse({ generatorId: "g1", capturedAt: "2026-09-06T12:00:00Z", communication: "online", metrics: { "generator.power_kw": { value: 0, unit: "kW", quality: "good", observedAt: "2026-09-06T12:00:00Z" }, "controller.mode": { value: "AUTO", quality: "good", observedAt: "2026-09-06T12:00:00Z" }, "breaker.gcb": { value: false, quality: "good", observedAt: "2026-09-06T12:00:00Z" } } });
    expect(parsed.metrics["generator.power_kw"]?.value).toBe(0);
    expect(parsed.metrics["breaker.gcb"]?.value).toBe(false);
  });
});

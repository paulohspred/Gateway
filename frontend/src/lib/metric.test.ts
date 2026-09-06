import { describe, expect, it } from "vitest";
import type { Telemetry } from "../api/schemas";
import { metricView } from "./metric";

const base: Telemetry = { generatorId: "g1", capturedAt: "2026-09-06T12:00:00Z", communication: "online", metrics: {} };

describe("metricView", () => {
  it("preserva zero real", () => {
    const telemetry: Telemetry = { ...base, metrics: { "generator.power_kw": { value: 0, unit: "kW", quality: "good", observedAt: "2026-09-06T12:00:00Z" } } };
    expect(metricView(telemetry, "generator.power_kw").display).toBe("0");
  });
  it("não converte ausência em zero", () => { expect(metricView(base, "fuel.level")).toEqual({ present: false, display: "N/D", quality: "missing" }); });
  it("não usa número bad como decisão visual", () => {
    const telemetry: Telemetry = { ...base, metrics: { "engine.rpm": { value: 1800, unit: "rpm", quality: "bad", observedAt: "2026-09-06T12:00:00Z" } } };
    expect(metricView(telemetry, "engine.rpm").display).toBe("Dado inválido");
  });
  it("preserva último valor stale com qualidade", () => {
    const telemetry: Telemetry = { ...base, metrics: { "engine.rpm": { value: 1800, unit: "rpm", quality: "stale", observedAt: "2026-09-06T11:59:00Z" } } };
    const view = metricView(telemetry, "engine.rpm", { digits: 0 });
    expect(view.display).toBe("1.800");
    expect(view.quality).toBe("stale");
  });
});

import type { Telemetry } from "../api/schemas";
import { metricOf } from "./metric";

export function explicitSourceState(telemetry: Telemetry | null | undefined, keys: readonly string[]): boolean | null {
  for (const key of keys) {
    const metric = metricOf(telemetry, key);
    if (!metric || metric.quality !== "good") continue;
    if (typeof metric.value === "boolean") return metric.value;
    if (typeof metric.value === "number") return metric.value !== 0;
    const value = metric.value.trim().toLowerCase();
    if (["on", "online", "running", "run", "active", "available", "ready", "ok", "supplying", "energized", "energised"].includes(value)) return true;
    if (["off", "offline", "stopped", "stop", "inactive", "unavailable", "idle", "failed", "fault"].includes(value)) return false;
  }
  return null;
}

export function explicitBreakerState(telemetry: Telemetry | null | undefined, key: string): boolean | null {
  const metric = metricOf(telemetry, key);
  return metric && metric.quality === "good" && typeof metric.value === "boolean" ? metric.value : null;
}

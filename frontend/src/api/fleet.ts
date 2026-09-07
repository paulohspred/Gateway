import { api } from "./client";
import type { Alarm, Generator, GeneratorCapabilities, Telemetry } from "./schemas";

export type FleetRow = {
  generator: Generator;
  telemetry: Telemetry | null;
  alarms: Alarm[];
  capabilities: GeneratorCapabilities | null;
  telemetryError?: string;
  alarmError?: string;
};

async function mapLimit<T, R>(items: readonly T[], limit: number, fn: (item: T) => Promise<R>): Promise<R[]> {
  const output = new Array<R>(items.length);
  let cursor = 0;
  async function worker() {
    for (;;) {
      const index = cursor++;
      const item = items[index];
      if (item === undefined) return;
      output[index] = await fn(item);
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, () => worker()));
  return output;
}

export async function getFleetRows(includeAlarms = true): Promise<FleetRow[]> {
  const generators = await api.listGenerators();
  return mapLimit(generators, 8, async (generator) => {
    const [telemetryResult, alarmResult, capabilityResult] = await Promise.allSettled([
      api.getTelemetry(generator.id),
      includeAlarms ? api.getAlarms(generator.id) : Promise.resolve([] as Alarm[]),
      api.getCapabilities(generator.id)
    ]);
    return {
      generator,
      telemetry: telemetryResult.status === "fulfilled" ? telemetryResult.value : null,
      alarms: alarmResult.status === "fulfilled" ? alarmResult.value : [],
      capabilities: capabilityResult.status === "fulfilled" ? capabilityResult.value : null,
      ...(telemetryResult.status === "rejected" ? { telemetryError: telemetryResult.reason instanceof Error ? telemetryResult.reason.message : "Falha de telemetria" } : {}),
      ...(alarmResult.status === "rejected" ? { alarmError: alarmResult.reason instanceof Error ? alarmResult.reason.message : "Falha de alarmes" } : {})
    };
  });
}

export async function getFleetEvents() {
  const generators = await api.listGenerators();
  const batches = await mapLimit(generators, 8, async (generator) => {
    try { return await api.getEvents(generator.id); } catch { return []; }
  });
  return batches.flat().sort((a, b) => Date.parse(b.occurredAt) - Date.parse(a.occurredAt));
}

export function metricSupported(row: FleetRow, key: string): boolean | undefined {
  if (!row.capabilities) return undefined;
  return row.capabilities.metrics.some((metric) => metric.key === key);
}

import type { Metric, Telemetry } from "../api/schemas";

export type MetricView = {
  present: boolean;
  display: string;
  quality: Metric["quality"] | "missing";
  observedAt?: string;
  raw?: Metric["value"];
  unit?: string;
};

export function metricOf(telemetry: Telemetry | null | undefined, key: string): Metric | undefined {
  return telemetry?.metrics[key];
}

export function formatScalar(value: Metric["value"], digits = 1): string {
  if (typeof value === "number") return new Intl.NumberFormat("pt-BR", { maximumFractionDigits: digits }).format(value);
  if (typeof value === "boolean") return value ? "Sim" : "Não";
  return value;
}

export function metricView(telemetry: Telemetry | null | undefined, key: string, options: { digits?: number; booleanLabels?: readonly [string, string] } = {}): MetricView {
  const metric = metricOf(telemetry, key);
  if (!metric) return { present: false, display: "N/D", quality: "missing" };
  if (metric.quality === "bad") return { present: true, display: "Dado inválido", quality: metric.quality, observedAt: metric.observedAt, raw: metric.value, ...(metric.unit ? { unit: metric.unit } : {}) };
  if (metric.quality === "unknown") return { present: true, display: "N/D", quality: metric.quality, observedAt: metric.observedAt, raw: metric.value, ...(metric.unit ? { unit: metric.unit } : {}) };
  let display: string;
  if (typeof metric.value === "boolean" && options.booleanLabels) display = metric.value ? options.booleanLabels[0] : options.booleanLabels[1];
  else display = formatScalar(metric.value, options.digits ?? 1);
  return { present: true, display, quality: metric.quality, observedAt: metric.observedAt, raw: metric.value, ...(metric.unit ? { unit: metric.unit } : {}) };
}

export function usableQuality(quality: Metric["quality"] | "missing") {
  return quality === "good" || quality === "stale" || quality === "offline";
}

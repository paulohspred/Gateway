import type { Telemetry } from "../api/schemas";
import { metricView } from "../lib/metric";
import { ageLabel } from "../lib/time";
import { StatusBadge } from "./StatusBadge";

export function MetricValue({ telemetry, metricKey, digits = 1, booleanLabels }: { telemetry?: Telemetry | null; metricKey: string; digits?: number; booleanLabels?: readonly [string, string] }) {
  const metric = metricView(telemetry, metricKey, { digits, ...(booleanLabels ? { booleanLabels } : {}) });
  return <span className={`metric-value quality-${metric.quality}`}><strong>{metric.display}</strong>{metric.unit && metric.display !== "Dado inválido" && metric.display !== "N/D" ? <span>{metric.unit}</span> : null}{metric.quality !== "good" && metric.quality !== "missing" ? <StatusBadge tone={metric.quality}>{metric.quality.toUpperCase()}{metric.observedAt ? ` · ${ageLabel(metric.observedAt)}` : ""}</StatusBadge> : null}</span>;
}

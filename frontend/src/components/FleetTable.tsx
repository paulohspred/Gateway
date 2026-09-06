import { Link } from "react-router-dom";
import type { FleetRow } from "../api/fleet";
import { metricView } from "../lib/metric";
import { formatDateTime } from "../lib/time";
import { StatusBadge } from "./StatusBadge";

function alarmTone(row: FleetRow) {
  if (row.alarms.some((alarm) => alarm.active && alarm.severity === "critical")) return "critical";
  if (row.alarms.some((alarm) => alarm.active && alarm.severity === "warning")) return "warning";
  return "none";
}

export function FleetTable({ rows }: { rows: FleetRow[] }) {
  return <div className="table-wrap"><table className="data-table"><thead><tr><th>Gerador</th><th>Site</th><th>Controladora</th><th>Comunicação</th><th>RPM</th><th>kW</th><th>Hz</th><th>Alarmes</th><th>Captura</th></tr></thead><tbody>{rows.map((row) => {
    const rpm = metricView(row.telemetry, "engine.rpm", { digits: 0 });
    const kw = metricView(row.telemetry, "generator.power_kw", { digits: 1 });
    const hz = metricView(row.telemetry, "generator.frequency", { digits: 1 });
    const activeAlarms = row.alarms.filter((alarm) => alarm.active);
    const tone = alarmTone(row);
    return <tr key={row.generator.id}><td><Link className="asset-link" to={`/generators/${encodeURIComponent(row.generator.id)}`}><strong>{row.generator.name}</strong><span>{row.generator.id}</span></Link></td><td>{row.generator.siteId}</td><td>{row.generator.controller.manufacturer} {row.generator.controller.model}</td><td><StatusBadge tone={row.telemetry?.communication ?? "unknown"}>{(row.telemetry?.communication ?? "unknown").toUpperCase()}</StatusBadge></td><td>{rpm.display}{rpm.unit ? ` ${rpm.unit}` : ""}</td><td>{kw.display}{kw.unit ? ` ${kw.unit}` : ""}</td><td>{hz.display}{hz.unit ? ` ${hz.unit}` : ""}</td><td>{tone === "none" ? <span className="muted">0</span> : <StatusBadge tone={tone}>{activeAlarms.length}</StatusBadge>}</td><td>{formatDateTime(row.telemetry?.capturedAt)}</td></tr>;
  })}</tbody></table></div>;
}

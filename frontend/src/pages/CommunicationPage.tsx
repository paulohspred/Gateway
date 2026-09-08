import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { getFleetRows } from "../api/fleet";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function CommunicationPage() {
  const fleet = useQuery({ queryKey: ["fleet", "communication"], queryFn: () => getFleetRows(false), refetchInterval: 10_000 });
  const health = useQuery({ queryKey: ["system-health"], queryFn: api.getSystemHealth, refetchInterval: 10_000 });
  const rows = fleet.data ?? [];
  const qualityTotals = { good: 0, stale: 0, offline: 0, bad: 0, unknown: 0 };
  for (const row of rows) for (const metric of Object.values(row.telemetry?.metrics ?? {})) qualityTotals[metric.quality] += 1;
  return <>
    <Topbar title="Comunicação" subtitle="Provider, comunicação e qualidade por gerador" onRefresh={() => { void fleet.refetch(); void health.refetch(); }} refreshing={fleet.isFetching || health.isFetching}/>
    <div className="content-grid">
      <section className="asset-strip"><div><span>RC Monitor</span><StatusBadge tone={health.data?.status ?? "degraded"}>{(health.data?.status ?? "N/D").toUpperCase()}</StatusBadge></div><div><span>Provider</span><StatusBadge tone={health.data?.provider.status ?? "unavailable"}>{(health.data?.provider.status ?? "N/D").toUpperCase()}</StatusBadge></div><div><span>Online</span><strong>{rows.filter((r) => r.telemetry?.communication === "online").length}</strong></div><div><span>Offline/Unknown</span><strong>{rows.filter((r) => r.telemetry?.communication !== "online").length}</strong></div></section>
      <Panel title="Qualidade de métricas"><div className="quality-count-grid">{Object.entries(qualityTotals).map(([quality, count]) => <div key={quality}><StatusBadge tone={quality}>{quality.toUpperCase()}</StatusBadge><strong>{count}</strong></div>)}</div></Panel>
      {fleet.isError || health.isError ? <div className="error-banner">Falha parcial na observabilidade de comunicação.</div> : null}
      <Panel title="Estado por gerador"><div className="table-wrap"><table className="data-table"><thead><tr><th>Gerador</th><th>Site</th><th>Comunicação</th><th>Snapshot</th><th>Good</th><th>Stale</th><th>Offline</th><th>Bad</th><th>Unknown</th><th>Observação</th></tr></thead><tbody>{rows.length ? rows.map((row) => { const counts = { good: 0, stale: 0, offline: 0, bad: 0, unknown: 0 }; for (const metric of Object.values(row.telemetry?.metrics ?? {})) counts[metric.quality] += 1; return <tr key={row.generator.id}><td>{row.generator.name}</td><td>{row.generator.siteId}</td><td><StatusBadge tone={row.telemetry?.communication ?? "unknown"}>{(row.telemetry?.communication ?? "unknown").toUpperCase()}</StatusBadge></td><td>{formatDateTime(row.telemetry?.capturedAt)}</td><td>{counts.good}</td><td>{counts.stale}</td><td>{counts.offline}</td><td>{counts.bad}</td><td>{counts.unknown}</td><td>{row.telemetryError || "—"}</td></tr>; }) : <tr><td colSpan={10} className="empty-cell">Nenhum gerador disponível.</td></tr>}</tbody></table></div></Panel>
      <div className="info-banner">RSSI, VPN, modem e meio físico só aparecem quando houver API/evidência real. A UI não infere esses estados.</div>
    </div>
  </>;
}

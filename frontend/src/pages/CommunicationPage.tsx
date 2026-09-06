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
  return <><Topbar title="Comunicação" subtitle="Saúde do provider e observabilidade por gerador" onRefresh={() => { void fleet.refetch(); void health.refetch(); }} refreshing={fleet.isFetching || health.isFetching}/><div className="content-grid"><section className="asset-strip"><div><span>RC Monitor</span><StatusBadge tone={health.data?.status ?? "degraded"}>{(health.data?.status ?? "degraded").toUpperCase()}</StatusBadge></div><div><span>Provider</span><StatusBadge tone={health.data?.provider.status ?? "unavailable"}>{(health.data?.provider.status ?? "unavailable").toUpperCase()}</StatusBadge></div><div><span>Verificado</span><strong>{formatDateTime(health.data?.provider.checkedAt)}</strong></div><div><span>Mensagem</span><strong>{health.data?.provider.message || "—"}</strong></div></section><Panel title="Estado por gerador"><div className="table-wrap"><table className="data-table"><thead><tr><th>Gerador</th><th>Site</th><th>Comunicação</th><th>Snapshot</th><th>Observação</th></tr></thead><tbody>{(fleet.data ?? []).map((row) => <tr key={row.generator.id}><td>{row.generator.name}</td><td>{row.generator.siteId}</td><td><StatusBadge tone={row.telemetry?.communication ?? "unknown"}>{(row.telemetry?.communication ?? "unknown").toUpperCase()}</StatusBadge></td><td>{formatDateTime(row.telemetry?.capturedAt)}</td><td>{row.telemetryError || "—"}</td></tr>)}</tbody></table></div></Panel></div></>;
}

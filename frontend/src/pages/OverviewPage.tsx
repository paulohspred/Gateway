import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, CircuitBoard, RadioTower, Server } from "lucide-react";
import { api } from "../api/client";
import { getFleetRows } from "../api/fleet";
import { FleetTable } from "../components/FleetTable";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";

export function OverviewPage() {
  const fleet = useQuery({ queryKey: ["fleet", "overview"], queryFn: () => getFleetRows(true), refetchInterval: 15_000 });
  const health = useQuery({ queryKey: ["system-health"], queryFn: api.getSystemHealth, refetchInterval: 10_000 });
  const rows = fleet.data ?? [];
  const online = rows.filter((row) => row.telemetry?.communication === "online").length;
  const offline = rows.filter((row) => row.telemetry?.communication === "offline").length;
  const criticalRows = rows.filter((row) => row.alarms.some((alarm) => alarm.active && alarm.severity === "critical"));
  const warningRows = rows.filter((row) => row.alarms.some((alarm) => alarm.active && alarm.severity === "warning"));
  const commIssues = rows.filter((row) => row.telemetry?.communication !== "online" || Boolean(row.telemetryError));
  const provider = health.data?.provider.status ?? "unavailable";

  return <>
    <Topbar title="Visão Geral" subtitle="Situação operacional do parque" onRefresh={() => { void fleet.refetch(); void health.refetch(); }} refreshing={fleet.isFetching || health.isFetching}/>
    <div className="content-grid">
      {fleet.error || health.error ? <div className="error-banner">Falha parcial de atualização. O sistema não converte indisponibilidade em zero nem em estado saudável.</div> : null}
      <div className="kpi-grid">
        <div className="kpi"><CircuitBoard/><div><span>Geradores</span><strong>{fleet.isLoading ? "—" : rows.length}</strong></div></div>
        <div className="kpi"><RadioTower/><div><span>Online</span><strong>{fleet.isLoading ? "—" : online}</strong><small>{offline} offline</small></div></div>
        <div className="kpi"><AlertTriangle/><div><span>Críticos ativos</span><strong>{fleet.isLoading ? "—" : criticalRows.length}</strong><small>{warningRows.length} com alerta</small></div></div>
        <div className="kpi"><Server/><div><span>Provider</span><StatusBadge tone={provider}>{provider.toUpperCase()}</StatusBadge></div></div>
      </div>
      <div className="two-col">
        <Panel title={`Equipamentos críticos (${criticalRows.length})`}>{criticalRows.length ? <FleetTable rows={criticalRows.slice(0, 12)}/> : <p className="empty">Nenhum equipamento com alarme crítico ativo.</p>}</Panel>
        <Panel title={`Problemas de comunicação (${commIssues.length})`}>{commIssues.length ? <div className="compact-list">{commIssues.slice(0, 20).map((row) => <div key={row.generator.id}><div><strong>{row.generator.name}</strong><span>{row.generator.siteId}</span></div><StatusBadge tone={row.telemetry?.communication ?? "unknown"}>{(row.telemetry?.communication ?? "unknown").toUpperCase()}</StatusBadge></div>)}</div> : <p className="empty">Nenhum problema de comunicação na frota visível.</p>}</Panel>
      </div>
      <Panel title="Frota operacional">{fleet.isLoading ? <p className="loading">Carregando telemetria da frota…</p> : rows.length ? <FleetTable rows={rows}/> : <p className="empty">Nenhum gerador operacional disponível para seu escopo.</p>}</Panel>
    </div>
  </>;
}

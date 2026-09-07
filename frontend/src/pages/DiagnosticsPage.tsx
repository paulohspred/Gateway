import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { controlApi } from "../api/control";
import { getFleetRows } from "../api/fleet";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";

export function DiagnosticsPage() {
  const health = useQuery({ queryKey: ["system-health"], queryFn: api.getSystemHealth, refetchInterval: 10_000 });
  const system = useQuery({ queryKey: ["admin-system", "diagnostics"], queryFn: controlApi.system, refetchInterval: 10_000 });
  const fleet = useQuery({ queryKey: ["fleet", "diagnostics"], queryFn: () => getFleetRows(false), refetchInterval: 10_000 });
  const commissionings = useQuery({ queryKey: ["commissionings", "diagnostics"], queryFn: controlApi.commissionings, staleTime: 30_000 });
  const byMonitorId = new Map((commissionings.data ?? []).filter((c) => c.monitorGeneratorId).map((c) => [c.monitorGeneratorId as string, c]));
  const gatewayReady = system.data?.gateway?.ready;
  return <>
    <Topbar title="Diagnóstico" subtitle="Cadeia de observabilidade sem estados presumidos" onRefresh={() => { void health.refetch(); void system.refetch(); void fleet.refetch(); void commissionings.refetch(); }} refreshing={health.isFetching || system.isFetching || fleet.isFetching}/>
    <div className="content-grid"><Panel title="Cadeia por gerador"><div className="diagnostic-list">{(fleet.data ?? []).map((row) => {
      const commissioning = byMonitorId.get(row.generator.id);
      const transportGate = commissioning?.gates.TRANSPORT?.status;
      const controllerGate = commissioning?.gates.CONTROLLER?.status;
      return <article key={row.generator.id}><header><strong>{row.generator.name}</strong><span>{row.generator.id}</span></header><div className="diagnostic-chain">
        <div><span>Controller</span><StatusBadge tone={controllerGate === "PASS" ? "healthy" : controllerGate === "FAIL" ? "critical" : "unknown"}>{controllerGate ?? "N/D"}</StatusBadge></div><b>→</b>
        <div><span>Transport</span><StatusBadge tone={transportGate === "PASS" ? "healthy" : transportGate === "FAIL" ? "critical" : "unknown"}>{transportGate ?? "N/D"}</StatusBadge></div><b>→</b>
        <div><span>RC Gateway</span><StatusBadge tone={gatewayReady === true ? "healthy" : gatewayReady === false ? "critical" : "unknown"}>{gatewayReady === true ? "READY" : gatewayReady === false ? "NOT READY" : "N/D"}</StatusBadge></div><b>→</b>
        <div><span>Rapid provider</span><StatusBadge tone={health.data?.provider.status ?? "unavailable"}>{(health.data?.provider.status ?? "N/D").toUpperCase()}</StatusBadge></div><b>→</b>
        <div><span>RC Monitor</span><StatusBadge tone={health.data?.status ?? "degraded"}>{(health.data?.status ?? "N/D").toUpperCase()}</StatusBadge></div><b>→</b>
        <div><span>Gerador</span><StatusBadge tone={row.telemetry?.communication ?? "unknown"}>{(row.telemetry?.communication ?? "unknown").toUpperCase()}</StatusBadge></div>
      </div>{row.telemetryError ? <p className="form-error">{row.telemetryError}</p> : null}</article>;
    })}</div>{!fleet.data?.length ? <p className="empty">Nenhum gerador disponível.</p> : null}<p className="panel-note">Controller e Transport só recebem PASS a partir de evidência de commissioning. Gateway é consultado pelo endpoint loopback real; nenhuma camada é marcada saudável por constante de UI.</p></Panel></div>
  </>;
}

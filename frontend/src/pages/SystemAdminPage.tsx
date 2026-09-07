import { useQuery } from "@tanstack/react-query";
import { controlApi } from "../api/control";
import { api } from "../api/client";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function SystemAdminPage() {
  const control = useQuery({ queryKey: ["admin-system"], queryFn: controlApi.system, refetchInterval: 10_000 });
  const sessions = useQuery({ queryKey: ["admin-sessions"], queryFn: controlApi.sessions, refetchInterval: 30_000 });
  const monitor = useQuery({ queryKey: ["system-health"], queryFn: api.getSystemHealth, refetchInterval: 10_000 });
  const gateway = control.data?.gateway;
  return <>
    <Topbar title="Saúde do sistema" subtitle="Control plane e operação" onRefresh={() => { void control.refetch(); void sessions.refetch(); void monitor.refetch(); }} refreshing={control.isFetching || sessions.isFetching || monitor.isFetching}/>
    <div className="content-grid">
      <section className="asset-strip"><div><span>RC Admin</span><StatusBadge tone={control.data ? "healthy" : "unavailable"}>{control.data ? "READY" : "N/D"}</StatusBadge></div><div><span>RC Gateway</span><StatusBadge tone={gateway?.ready === true ? "healthy" : gateway?.ready === false ? "critical" : "unknown"}>{gateway?.operationalState?.toUpperCase() ?? "N/D"}</StatusBadge></div><div><span>RC Monitor</span><StatusBadge tone={monitor.data?.status ?? "unavailable"}>{(monitor.data?.status ?? "N/D").toUpperCase()}</StatusBadge></div><div><span>Rapid provider</span><StatusBadge tone={monitor.data?.provider.status ?? "unavailable"}>{(monitor.data?.provider.status ?? "N/D").toUpperCase()}</StatusBadge></div></section>
      {control.isError || monitor.isError ? <div className="error-banner">Algum componente de observabilidade está indisponível.</div> : null}
      <div className="two-col"><Panel title="Sessões"><div className="metric-grid"><div className="metric-cell"><span>Ativas</span><strong>{sessions.data?.length ?? "—"}</strong></div><div className="metric-cell"><span>TTL</span><strong>{control.data ? `${control.data.sessionTtlSeconds / 3600} h` : "N/D"}</strong></div><div className="metric-cell"><span>Cookie Secure</span><strong>{control.data ? String(control.data.cookieSecure) : "N/D"}</strong></div><div className="metric-cell"><span>Gateway sessions</span><strong>{gateway?.activeSessions ?? "N/D"}</strong></div></div></Panel><Panel title="Provider"><p>{monitor.data?.provider.message || "N/D"}</p><small>Última verificação: {formatDateTime(monitor.data?.provider.checkedAt)}</small></Panel></div>
      <Panel title="Versões"><div className="technical-grid"><div><span>Release</span><strong>{control.data?.releaseVersion || "N/D"}</strong></div><div><span>Commit</span><strong>{control.data?.commit || "N/D"}</strong></div><div><span>Rapid SCADA</span><strong>{control.data?.rapidScadaVersion || "N/D"}</strong></div><div><span>Go</span><strong>{control.data?.goVersion || "N/D"}</strong></div><div><span>Node</span><strong>{control.data?.nodeVersion || "N/D"}</strong></div><div><span>Build</span><strong>{control.data?.buildDate || "N/D"}</strong></div></div></Panel>
    </div>
  </>;
}

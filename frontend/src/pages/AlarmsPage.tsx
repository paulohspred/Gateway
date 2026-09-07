import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getFleetRows } from "../api/fleet";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function AlarmsPage() {
  const [severity, setSeverity] = useState("all");
  const [state, setState] = useState("active");
  const [query, setQuery] = useState("");
  const fleet = useQuery({ queryKey: ["fleet", "alarms"], queryFn: () => getFleetRows(true), refetchInterval: 10_000 });
  const alarms = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("pt-BR");
    return (fleet.data ?? []).flatMap(({ generator, alarms }) => alarms.map((alarm) => ({ generator, alarm }))).filter(({ generator, alarm }) => {
      if (severity !== "all" && alarm.severity !== severity) return false;
      if (state === "active" && !alarm.active) return false;
      if (state === "cleared" && alarm.active) return false;
      if (!needle) return true;
      return [generator.name, generator.id, generator.siteId, alarm.code, alarm.message].some((value) => value.toLocaleLowerCase("pt-BR").includes(needle));
    }).sort((a, b) => Date.parse(b.alarm.raisedAt) - Date.parse(a.alarm.raisedAt));
  }, [fleet.data, severity, state, query]);
  const counts = (fleet.data ?? []).flatMap((row) => row.alarms).filter((alarm) => alarm.active);
  return <>
    <Topbar title="Alarmes" subtitle="Condições retornadas pelo RC Monitor; ACK permanece desabilitado" onRefresh={() => void fleet.refetch()} refreshing={fleet.isFetching}/>
    <div className="content-grid">
      <section className="asset-strip"><div><span>Críticos ativos</span><strong>{counts.filter((a) => a.severity === "critical").length}</strong></div><div><span>Alertas ativos</span><strong>{counts.filter((a) => a.severity === "warning").length}</strong></div><div><span>Informativos ativos</span><strong>{counts.filter((a) => a.severity === "info").length}</strong></div><div><span>Total filtrado</span><strong>{alarms.length}</strong></div></section>
      {fleet.isError ? <div className="error-banner">{fleet.error instanceof Error ? fleet.error.message : "Falha ao carregar alarmes"}</div> : null}
      <Panel title="Alarmes" action={<div className="filter-row"><select className="filter-input" value={state} onChange={(e) => setState(e.target.value)}><option value="active">Ativos</option><option value="cleared">Normalizados</option><option value="all">Todos retornados</option></select><select className="filter-input" value={severity} onChange={(e) => setSeverity(e.target.value)}><option value="all">Todas severidades</option><option value="critical">Crítico</option><option value="warning">Alerta</option><option value="info">Info</option></select><input className="filter-input" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Gerador, site, código ou mensagem"/></div>}>
        <div className="table-wrap"><table className="data-table"><thead><tr><th>Gerador</th><th>Site</th><th>Estado</th><th>Severidade</th><th>Código</th><th>Mensagem</th><th>Desde</th><th>Normalizado</th></tr></thead><tbody>{alarms.length ? alarms.map(({ generator, alarm }) => <tr key={`${generator.id}-${alarm.id}`}><td>{generator.name}</td><td>{generator.siteId}</td><td><StatusBadge tone={alarm.active ? "warning" : "healthy"}>{alarm.active ? "ATIVO" : "NORMAL"}</StatusBadge></td><td><StatusBadge tone={alarm.severity}>{alarm.severity.toUpperCase()}</StatusBadge></td><td><code>{alarm.code}</code></td><td>{alarm.message}</td><td>{formatDateTime(alarm.raisedAt)}</td><td>{formatDateTime(alarm.clearedAt)}</td></tr>) : <tr><td colSpan={8} className="empty-cell">Nenhum alarme corresponde aos filtros.</td></tr>}</tbody></table></div>
      </Panel>
    </div>
  </>;
}

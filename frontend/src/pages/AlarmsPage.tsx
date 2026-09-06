import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getFleetRows } from "../api/fleet";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function AlarmsPage() {
  const [severity, setSeverity] = useState("all");
  const fleet = useQuery({ queryKey: ["fleet", "alarms"], queryFn: () => getFleetRows(true), refetchInterval: 10_000 });
  const alarms = useMemo(() => (fleet.data ?? []).flatMap(({ generator, alarms }) => alarms.filter((alarm) => alarm.active).map((alarm) => ({ generator, alarm }))).filter(({ alarm }) => severity === "all" || alarm.severity === severity).sort((a, b) => Date.parse(b.alarm.raisedAt) - Date.parse(a.alarm.raisedAt)), [fleet.data, severity]);
  return <><Topbar title="Alarmes" subtitle="Condições ativas retornadas pelo RC Monitor" onRefresh={() => void fleet.refetch()} refreshing={fleet.isFetching}/><div className="content-grid"><Panel title={`Alarmes ativos (${alarms.length})`} action={<select className="filter-input" value={severity} onChange={(event) => setSeverity(event.target.value)}><option value="all">Todas as severidades</option><option value="critical">Crítico</option><option value="warning">Alerta</option><option value="info">Info</option></select>}><div className="table-wrap"><table className="data-table"><thead><tr><th>Gerador</th><th>Severidade</th><th>Código</th><th>Mensagem</th><th>Desde</th></tr></thead><tbody>{alarms.map(({ generator, alarm }) => <tr key={`${generator.id}-${alarm.id}`}><td>{generator.name}</td><td><StatusBadge tone={alarm.severity}>{alarm.severity.toUpperCase()}</StatusBadge></td><td>{alarm.code}</td><td>{alarm.message}</td><td>{formatDateTime(alarm.raisedAt)}</td></tr>)}</tbody></table></div></Panel></div></>;
}

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { getFleetEvents } from "../api/fleet";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function EventsPage() {
  const [query, setQuery] = useState("");
  const [type, setType] = useState("all");
  const [periodHours, setPeriodHours] = useState(24);
  const events = useQuery({ queryKey: ["fleet-events"], queryFn: getFleetEvents, refetchInterval: 10_000 });
  const generators = useQuery({ queryKey: ["generators", "event-labels"], queryFn: api.listGenerators, staleTime: 60_000 });
  const labels = useMemo(() => new Map((generators.data ?? []).map((g) => [g.id, `${g.name} · ${g.siteId}`])), [generators.data]);
  const types = useMemo(() => Array.from(new Set((events.data ?? []).map((e) => e.type))).sort(), [events.data]);
  const rows = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("pt-BR");
    const cutoff = Date.now() - periodHours * 3600_000;
    return (events.data ?? []).filter((event) => Date.parse(event.occurredAt) >= cutoff).filter((event) => type === "all" || event.type === type).filter((event) => !needle || [event.generatorId, labels.get(event.generatorId) ?? "", event.type, event.message].some((v) => v.toLocaleLowerCase("pt-BR").includes(needle)));
  }, [events.data, labels, periodHours, query, type]);
  return <>
    <Topbar title="Eventos" subtitle="Linha temporal factual recebida do Rapid via RC Monitor" onRefresh={() => { void events.refetch(); void generators.refetch(); }} refreshing={events.isFetching}/>
    <div className="content-grid">
      {events.isError ? <div className="error-banner">{events.error instanceof Error ? events.error.message : "Histórico recente de eventos indisponível"}</div> : null}
      <Panel title={`Eventos (${rows.length})`} action={<div className="filter-row"><select className="filter-input" value={periodHours} onChange={(e) => setPeriodHours(Number(e.target.value))}><option value={1}>1 hora</option><option value={6}>6 horas</option><option value={24}>24 horas</option><option value={168}>7 dias</option></select><select className="filter-input" value={type} onChange={(e) => setType(e.target.value)}><option value="all">Todos os tipos</option>{types.map((value) => <option key={value} value={value}>{value}</option>)}</select><input className="filter-input" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Gerador, tipo ou mensagem"/></div>}>
        <div className="table-wrap"><table className="data-table"><thead><tr><th>Hora</th><th>Gerador / Site</th><th>Tipo</th><th>Mensagem</th></tr></thead><tbody>{rows.length ? rows.slice(0, 500).map((event) => <tr key={event.id}><td>{formatDateTime(event.occurredAt)}</td><td>{labels.get(event.generatorId) ?? event.generatorId}</td><td><code>{event.type}</code></td><td>{event.message}</td></tr>) : <tr><td colSpan={4} className="empty-cell">Nenhum evento retornado para o período/filtro.</td></tr>}</tbody></table></div>
      </Panel>
    </div>
  </>;
}

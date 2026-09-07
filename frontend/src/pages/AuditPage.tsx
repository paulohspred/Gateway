import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { controlApi } from "../api/control";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

export function AuditPage() {
  const query = useQuery({ queryKey: ["audit"], queryFn: controlApi.audit, refetchInterval: 15_000 });
  const [search, setSearch] = useState("");
  const [result, setResult] = useState("all");
  const [action, setAction] = useState("all");
  const actions = useMemo(() => Array.from(new Set((query.data ?? []).map((event) => event.action))).sort(), [query.data]);
  const rows = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase("pt-BR");
    return (query.data ?? []).filter((event) => result === "all" || event.result === result).filter((event) => action === "all" || event.action === action).filter((event) => {
      if (!needle) return true;
      const details = event.details ? Object.entries(event.details).map(([key, value]) => `${key}=${value}`).join(" ") : "";
      return [event.actorUsername ?? "system", event.action, event.objectType, event.objectId ?? "", event.correlationId ?? "", event.source ?? "", details].some((value) => value.toLocaleLowerCase("pt-BR").includes(needle));
    });
  }, [query.data, search, result, action]);
  return <>
    <Topbar title="Auditoria" subtitle="Trilha administrativa, de segurança e commissioning" onRefresh={() => void query.refetch()} refreshing={query.isFetching}/>
    <div className="content-grid">
      {query.isError ? <div className="error-banner">{query.error instanceof Error ? query.error.message : "Auditoria indisponível"}</div> : null}
      <Panel title={`Eventos (${rows.length})`} action={<div className="filter-row"><select className="filter-input" value={result} onChange={(event) => setResult(event.target.value)}><option value="all">Todos resultados</option><option value="success">Sucesso</option><option value="failure">Falha</option></select><select className="filter-input" value={action} onChange={(event) => setAction(event.target.value)}><option value="all">Todas ações</option>{actions.map((value) => <option value={value} key={value}>{value}</option>)}</select><input className="filter-input" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Ator, objeto, correlation ID…"/></div>}>
        <div className="table-wrap"><table className="data-table audit-table"><thead><tr><th>Hora</th><th>Ator</th><th>Ação</th><th>Objeto</th><th>Resultado</th><th>Correlation</th><th>Origem</th><th>Detalhes</th></tr></thead><tbody>{rows.length ? rows.map((event) => <tr key={event.id}><td>{formatDateTime(event.at)}</td><td>{event.actorUsername || "system"}</td><td><code>{event.action}</code></td><td>{event.objectType}{event.objectId ? ` · ${event.objectId}` : ""}</td><td><StatusBadge tone={event.result === "success" ? "healthy" : "critical"}>{event.result.toUpperCase()}</StatusBadge></td><td><code>{event.correlationId || "—"}</code></td><td>{event.source || "—"}</td><td>{event.details ? Object.entries(event.details).map(([key, value]) => `${key}=${value}`).join(" · ") : "—"}</td></tr>) : <tr><td colSpan={8} className="empty-cell">Nenhum evento corresponde aos filtros.</td></tr>}</tbody></table></div>
      </Panel>
    </div>
  </>;
}

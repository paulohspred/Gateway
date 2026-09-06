import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getFleetRows } from "../api/fleet";
import { FleetTable } from "../components/FleetTable";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";

export function GeneratorsPage() {
  const [query, setQuery] = useState("");
  const fleet = useQuery({ queryKey: ["fleet", "generators"], queryFn: () => getFleetRows(true), refetchInterval: 15_000 });
  const rows = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("pt-BR");
    if (!needle) return fleet.data ?? [];
    return (fleet.data ?? []).filter(({ generator }) => [generator.name, generator.id, generator.siteId, generator.controller.manufacturer, generator.controller.model].some((value) => value.toLocaleLowerCase("pt-BR").includes(needle)));
  }, [fleet.data, query]);
  return <><Topbar title="Geradores" subtitle="Ativos com dados fornecidos pelo RC Monitor" onRefresh={() => void fleet.refetch()} refreshing={fleet.isFetching}/><div className="content-grid"><Panel title="Lista de geradores" action={<input className="filter-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar nome, ID, site ou controladora"/>}>{fleet.isLoading ? <p className="loading">Carregando…</p> : <FleetTable rows={rows}/>}</Panel></div></>;
}

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { LayoutGrid, List, Rows3 } from "lucide-react";
import { getFleetRows } from "../api/fleet";
import { controlApi } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { FleetTable } from "../components/FleetTable";
import { GeneratorCompactCard } from "../components/GeneratorCompactCard";
import { GeneratorVerticalCard } from "../components/GeneratorVerticalCard";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";

type FleetView = "vertical" | "compact" | "list";
type FleetFilter = "all" | "online" | "offline" | "alarm";

export function GeneratorsPage() {
  const [query, setQuery] = useState("");
  const auth = useAuth();
  const initialView = auth.user?.preferences?.fleetView as FleetView | undefined;
  const [view, setView] = useState<FleetView>(initialView ?? "vertical");
  const [filter, setFilter] = useState<FleetFilter>("all");
  const fleet = useQuery({ queryKey: ["fleet", "generators"], queryFn: () => getFleetRows(true), refetchInterval: 15_000 });
  const rows = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase("pt-BR");
    return (fleet.data ?? []).filter((row) => {
      const matchesText = !needle || [row.generator.name,row.generator.id,row.generator.siteId,row.generator.controller.manufacturer,row.generator.controller.model].some((value) => value.toLocaleLowerCase("pt-BR").includes(needle));
      const matchesFilter = filter === "all" || (filter === "online" && row.telemetry?.communication === "online") || (filter === "offline" && row.telemetry?.communication === "offline") || (filter === "alarm" && row.alarms.some((alarm) => alarm.active));
      return matchesText && matchesFilter;
    });
  }, [fleet.data, query, filter]);
  const setFleetView = (next: FleetView) => {
    setView(next);
    const prefs = auth.user?.preferences;
    if (prefs) void controlApi.updatePreferences(auth.csrf, { ...prefs, fleetView: next }).then(() => auth.refresh()).catch(() => undefined);
  };

  const controls = <div className="fleet-toolbar"><div className="fleet-view-switch" aria-label="Visualização da frota"><button className={view === "vertical" ? "active" : ""} onClick={() => setFleetView("vertical")}><Rows3/>Vertical</button><button className={view === "compact" ? "active" : ""} onClick={() => setFleetView("compact")}><LayoutGrid/>Compacto</button><button className={view === "list" ? "active" : ""} onClick={() => setFleetView("list")}><List/>Lista</button></div><select className="fleet-filter" value={filter} onChange={(e) => setFilter(e.target.value as FleetFilter)}><option value="all">Todos</option><option value="online">Online</option><option value="offline">Offline</option><option value="alarm">Com alarme</option></select><input className="filter-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar nome, ID, site ou controladora"/></div>;

  return <><Topbar title="Geradores" subtitle="Frota operacional · visualização vertical, compacta ou em lista" onRefresh={() => void fleet.refetch()} refreshing={fleet.isFetching}/><div className="content-grid fleet-page"><Panel title={`Frota de geradores (${rows.length})`} action={controls}>{fleet.isLoading ? <p className="loading">Carregando…</p> : fleet.isError ? <div className="error-banner">{fleet.error instanceof Error ? fleet.error.message : "Falha ao carregar frota"}</div> : rows.length === 0 ? <p className="empty">Nenhum gerador corresponde ao filtro atual.</p> : view === "vertical" ? <div className="generator-vertical-grid">{rows.map(row => <GeneratorVerticalCard row={row} key={row.generator.id}/>)}</div> : view === "compact" ? <div className="compact-generator-grid">{rows.map(row => <GeneratorCompactCard row={row} key={row.generator.id}/>)}</div> : <FleetTable rows={rows}/>}</Panel></div></>;
}

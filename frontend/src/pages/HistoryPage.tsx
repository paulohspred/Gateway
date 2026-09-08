import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, RefreshCw } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

const ranges = { "1h": 60 * 60 * 1000, "6h": 6 * 60 * 60 * 1000, "24h": 24 * 60 * 60 * 1000, "7d": 7 * 24 * 60 * 60 * 1000 } as const;
function archiveBit(range: keyof typeof ranges) { return range === "7d" ? 2 : 1; }
function MiniTrend({ name, unit, points }: { name: string; unit?: string; points: { timestamp: string; value: number; quality: string }[] }) {
  if (!points.length) return <article className="trend-card"><header><strong>{name}</strong><span>{unit || "—"}</span></header><p className="empty">Sem amostras arquivadas neste período.</p></article>;
  const values = points.map(p => p.value); const min = Math.min(...values); const max = Math.max(...values); const span = max - min || 1;
  const coords = points.map((p, i) => `${8 + (i / Math.max(1, points.length - 1)) * 584},${112 - ((p.value - min) / span) * 92}`).join(" ");
  const last = points[points.length - 1];
  return <article className="trend-card"><header><div><strong>{name}</strong><small>{points.length} amostras</small></div><div><b>{last?.value.toLocaleString("pt-BR", { maximumFractionDigits: 2 })}</b><span>{unit || ""}</span></div></header><svg viewBox="0 0 600 124" role="img" aria-label={`Tendência ${name}`}><line x1="8" y1="112" x2="592" y2="112" className="trend-axis"/><polyline points={coords} className="trend-line" fill="none"/></svg><footer><span>{formatDateTime(points[0]?.timestamp)}</span><span>{min.toLocaleString("pt-BR", { maximumFractionDigits: 2 })}–{max.toLocaleString("pt-BR", { maximumFractionDigits: 2 })}</span><span>{formatDateTime(last?.timestamp)}</span></footer></article>;
}
export function HistoryPage() {
  const [searchParams] = useSearchParams();
  const requestedGenerator = searchParams.get("generator") ?? "";
  const generators = useQuery({ queryKey: ["history-generators"], queryFn: api.listGenerators });
  const [generatorId, setGeneratorId] = useState(requestedGenerator); const [range, setRange] = useState<keyof typeof ranges>("6h"); const [selected, setSelected] = useState<string[]>([]); const [refresh, setRefresh] = useState(0);
  useEffect(() => { if (!generatorId && generators.data?.[0]) setGeneratorId(generators.data[0].id); }, [generators.data, generatorId]);
  const capabilities = useQuery({ queryKey: ["history-capabilities", generatorId], queryFn: () => api.getCapabilities(generatorId), enabled: Boolean(generatorId) });
  const numeric = useMemo(() => capabilities.data?.metrics.filter(m => m.kind === "number") ?? [], [capabilities.data]);
  useEffect(() => { if (numeric.length && selected.length === 0) setSelected(numeric.slice(0, 3).map(m => m.key)); }, [numeric, selected.length]);
  const query = useQuery({ queryKey: ["history", generatorId, range, selected.join(","), refresh], enabled: Boolean(generatorId && selected.length), retry: false, queryFn: async () => { const end = new Date(); const start = new Date(end.getTime() - ranges[range]); return api.getHistory(generatorId, selected, start, end, archiveBit(range)); } });
  const toggle = (key: string) => setSelected(prev => prev.includes(key) ? prev.filter(x => x !== key) : prev.length < 4 ? [...prev, key] : prev);
  return <><Topbar title="Histórico e Tendências" subtitle="Archive real do Rapid SCADA — nenhuma série é fabricada" onRefresh={() => setRefresh(x => x + 1)} refreshing={query.isFetching}><button className="icon-button" onClick={() => setRefresh(x => x + 1)} aria-label="Atualizar"><RefreshCw/></button></Topbar><div className="content-grid"><Panel title="Consulta"><div className="history-controls"><label>Gerador<select value={generatorId} onChange={e => { setGeneratorId(e.target.value); setSelected([]); }}>{generators.data?.map(g => <option key={g.id} value={g.id}>{g.name} · {g.siteId}</option>)}</select></label><label>Período<select value={range} onChange={e => setRange(e.target.value as keyof typeof ranges)}>{Object.keys(ranges).map(r => <option key={r} value={r}>{r}</option>)}</select></label><fieldset><legend>Métricas (máx. 4)</legend>{numeric.map(m => <label key={m.key} className="check-row"><input type="checkbox" checked={selected.includes(m.key)} onChange={() => toggle(m.key)}/>{m.displayName}{m.required ? " *" : ""}</label>)}</fieldset></div></Panel>{query.error ? <div className="warning-banner">Histórico indisponível para esta consulta. Isso pode significar que o archive Rapid ainda não está configurado para estes canais; nenhum dado alternativo foi inventado.</div> : null}<Panel title="Tendências" action={<Activity/>}>{query.isLoading ? <p className="loading">Consultando archive…</p> : <div className="trend-grid">{query.data?.series.map(series => <MiniTrend key={series.metricKey} name={numeric.find(x => x.key === series.metricKey)?.displayName || series.metricKey} unit={series.unit} points={series.points}/>)}</div>}</Panel></div></>;
}

import { RadioTower } from "lucide-react";
import { Link } from "react-router-dom";
import { metricSupported, type FleetRow } from "../api/fleet";
import { CONTROLLER_IMAGE_FALLBACK, controllerImageSrc } from "../lib/controllerImage";
import { metricView } from "../lib/metric";
import { formatDateTime } from "../lib/time";
import { IconBattery, IconBolt, IconFuelPump, IconRunHours } from "./ScadaIcons";
import { StatusBadge } from "./StatusBadge";

function Metric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) { return <div className="compact-metric"><span>{icon}</span><em>{label}</em><strong>{value}</strong></div>; }
function text(row: FleetRow, key: string, digits = 1) { const v = metricView(row.telemetry, key, { digits, supported: metricSupported(row, key) }); return !v.present || ["N/D","Dado inválido"].includes(v.display) ? v.display : `${v.display}${v.unit ? ` ${v.unit}` : ""}`; }

export function GeneratorCompactCard({ row }: { row: FleetRow }) {
  const mode = metricView(row.telemetry, "controller.mode", { supported: metricSupported(row, "controller.mode") });
  const active = row.alarms.filter((alarm) => alarm.active);
  return <article className={`compact-generator-card communication-${row.telemetry?.communication ?? "unknown"}`}>
    <header><div><h3>{row.generator.name}</h3><p>{row.generator.id} · {row.generator.controller.model}</p></div><div className="compact-status"><StatusBadge tone={row.telemetry?.communication ?? "unknown"}>{(row.telemetry?.communication ?? "unknown").toUpperCase()}</StatusBadge>{active.length ? <StatusBadge tone={active.some(a => a.severity === "critical") ? "critical" : "warning"}>{active.length}</StatusBadge> : null}</div></header>
    <div className="compact-body"><div className="controller-image-area"><img src={controllerImageSrc(row.generator.controller.model)} alt={`${row.generator.controller.manufacturer} ${row.generator.controller.model}`} onError={(event) => { event.currentTarget.src = CONTROLLER_IMAGE_FALLBACK; }}/></div><div className="compact-metrics"><Metric icon={<RadioTower size={12}/>} label="Comunicação" value={(row.telemetry?.communication ?? "unknown").toUpperCase()}/><Metric icon={<IconBattery size={12}/>} label="Bateria" value={text(row,"battery.voltage",1)}/><Metric icon={<IconBolt size={12}/>} label="Frequência" value={text(row,"generator.frequency",1)}/><Metric icon={<IconRunHours size={12}/>} label="Tempo operação" value={text(row,"engine.run_hours",1)}/><Metric icon={<IconFuelPump size={12}/>} label="Combustível" value={text(row,"fuel.level",0)}/><Metric icon={<RadioTower size={12}/>} label="Captura" value={formatDateTime(row.telemetry?.capturedAt)}/></div></div>
    <p className="compact-mode">Modo <b>{mode.display}</b></p>
    {row.telemetry?.communication === "offline" ? <p className="compact-stale">Valores da última leitura conhecida</p> : null}
    <Link className="compact-open" to={`/generators/${encodeURIComponent(row.generator.id)}`}>Abrir gerador ↗</Link>
  </article>;
}

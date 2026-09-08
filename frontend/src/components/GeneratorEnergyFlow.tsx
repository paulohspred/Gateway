import type { Telemetry } from "../api/schemas";
import { explicitBreakerState, explicitSourceState } from "../lib/electricalState";
import { metricView } from "../lib/metric";

function valueText(telemetry: Telemetry | null | undefined, key: string, digits = 1) {
  const view = metricView(telemetry, key, { digits });
  if (!view.present || view.display === "N/D" || view.display === "Dado inválido") return view.display;
  return `${view.display}${view.unit ? ` ${view.unit}` : ""}`;
}

function Switch({ y, closed, label }: { y: number; closed: boolean | null; label: string }) {
  const unknown = closed === null;
  return <g className={unknown ? "flow-switch is-unknown" : "flow-switch"}>
    <text x="34" y={y + 5} className="flow-breaker-name">{label}</text>
    <circle cx="80" cy={y - 15} r="4" className="flow-switch-node"/>
    <circle cx="80" cy={y + 15} r="4" className="flow-switch-node"/>
    <line x1="80" y1={y - 15} x2={closed === true ? 80 : 100} y2={closed === true ? y + 15 : y + 5} className="flow-switch-blade"/>
    <text x="112" y={y + 5} className="flow-breaker-state">{closed === null ? "N/D" : closed ? "FECHADO" : "ABERTO"}</text>
  </g>;
}

export function GeneratorEnergyFlow({ telemetry, compact = false }: { telemetry?: Telemetry | null; compact?: boolean }) {
  const mains = explicitSourceState(telemetry, ["mains.state"]);
  const generator = explicitSourceState(telemetry, ["generator.status", "engine.state"]);
  const mcb = explicitBreakerState(telemetry, "breaker.mcb");
  const gcb = explicitBreakerState(telemetry, "breaker.gcb");
  const mainsBus = mains === true && mcb === true;
  const generatorBus = generator === true && gcb === true;
  const bus = mainsBus || generatorBus;
  const gridHz = valueText(telemetry, "mains.frequency", 1);
  const genHz = valueText(telemetry, "generator.frequency", 1);
  const load = valueText(telemetry, "generator.power_kw", 0);

  return <svg viewBox="0 0 230 400" className={`flow-diagram ${compact ? "is-compact" : ""}`} preserveAspectRatio="xMidYMid meet" role="img" aria-label="Fluxo de energia read-only">
    <g transform="translate(80 40)">
      <circle r="28" className={`flow-device-circle ${mains === true ? "source-active" : mains === false ? "source-off" : "source-unknown"}`}/>
      <g className="flow-device-icon tower-icon"><path d="M0-20 0 18M-8 18 0-20 8 18M-12-8H12M-15 2H15M-18 12H18"/><path d="m-11-8 11 10 11-10M-14 2 0 12 14 2"/></g>
      <text x="35" y="5" className="flow-source-value">{gridHz}</text>
      <text x="0" y="-36" textAnchor="middle" className="flow-source-label">REDE</text>
    </g>

    <g transform="translate(178 200)">
      <rect x="-46" y="-32" width="90" height="64" rx="6" className={`flow-load-card ${bus ? "is-live" : ""}`}/>
      <g transform="translate(-22 0)" className="flow-device-icon"><path d="M-16 10 h32"/><path d="M-12 10 V-2 l 7 2 V-12 l 8 2 V10 M 3 -1 l 7 2 V10"/><path d="M-8.5 3 v0 M-8.5 7 v0 M -1 -4 v0 M -1 2 v0 M -1 8 v0 M 6.5 4 v0 M 6.5 8 v0" strokeWidth="2.5" strokeLinecap="round"/></g>
      <text x="16" y="-7" textAnchor="middle" className="flow-load-card-title">LOAD</text>
      <text x="16" y="14" textAnchor="middle" className="flow-load-card-value">{load}</text>
    </g>

    <g transform="translate(80 360)">
      <circle r="34" className={`flow-device-circle flow-generator-circle ${generator === true ? "generator-running" : generator === false ? "generator-stopped" : "source-unknown"}`}/>
      <text x="0" y="12" textAnchor="middle" className="flow-generator-letter">G</text>
      <text x="-42" y="5" textAnchor="end" className="flow-source-value">{genHz}</text>
      <text x="0" y="53" textAnchor="middle" className="flow-source-label">GERADOR</text>
    </g>

    <path d="M80 68 V95" className={`flow-bus-base ${mains === true ? "is-live" : ""}`}/>
    <path d="M80 145 V200" className={`flow-bus-base ${mainsBus ? "is-live" : ""}`}/>
    <path d="M80 200 V255" className={`flow-bus-base ${generatorBus ? "is-live" : ""}`}/>
    <path d="M80 305 V326" className={`flow-bus-base ${generator === true ? "is-live" : ""}`}/>
    <path d="M80 200 H132" className={`flow-bus-base ${bus ? "is-live" : ""}`}/>
    <Switch y={120} closed={mcb} label="MCB"/>
    <Switch y={280} closed={gcb} label="GCB"/>
    <circle cx="80" cy="200" r="5" className={`flow-junction ${bus ? "is-live" : ""}`}/>
  </svg>;
}

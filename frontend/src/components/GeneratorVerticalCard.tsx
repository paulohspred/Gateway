import { Link } from "react-router-dom";
import type { FleetRow } from "../api/fleet";
import { metricOf, metricView } from "../lib/metric";
import { GeneratorEnergyFlow } from "./GeneratorEnergyFlow";
import { IconBattery, IconBolt, IconFuelPump, IconHouse, IconOilCan, IconRunHours, IconThermometer } from "./ScadaIcons";

function metricText(row: FleetRow, key: string, digits = 1) {
  const view = metricView(row.telemetry, key, { digits });
  if (!view.present || view.display === "N/D" || view.display === "Dado inválido") return view.display;
  return `${view.display}${view.unit ? ` ${view.unit}` : ""}`;
}

function EngineRow({ icon, label, value, quality, percent }: { icon: React.ReactNode; label: string; value: string; quality: string; percent?: number | null }) {
  const width = typeof percent === "number" && Number.isFinite(percent) ? Math.min(100, Math.max(0, percent)) : 0;
  return <div className={`operator-engine-row quality-${quality}`}><span className="engine-icon">{icon}</span><span className="engine-label">{label}</span><span className={`engine-meter ${percent == null ? "is-unscaled" : ""}`}><i style={{ width: `${width}%` }}/></span><strong>{value}</strong></div>;
}

function PowerGauge({ row }: { row: FleetRow }) {
  const power = metricView(row.telemetry, "generator.power_kw", { digits: 0 });
  const raw = typeof power.raw === "number" && power.quality === "good" ? power.raw : null;
  const nominal = row.generator.spec?.ratedPowerKw;
  const scaled = raw != null && nominal != null && nominal > 0;
  const pct = scaled ? Math.min(1, Math.max(0, raw / nominal)) : 0;
  const angle = 180 + pct * 180;
  return <div className="operator-power-gauge">
    <svg viewBox="0 0 220 116" aria-label="Potência ativa"><path d="M25 96 A85 85 0 0 1 195 96" className="gauge-base"/><path d="M25 96 A85 85 0 0 1 156 29" className="gauge-good"/><path d="M156 29 A85 85 0 0 1 184 53" className="gauge-warning"/><path d="M184 53 A85 85 0 0 1 195 96" className="gauge-critical"/>{scaled ? <line x1="110" y1="96" x2="110" y2="28" className="gauge-needle" style={{ transform: `rotate(${angle - 270}deg)`, transformOrigin: "110px 96px" }}/> : null}<circle cx="110" cy="96" r="5" className="gauge-hub"/></svg>
    <strong>{power.display}{power.unit && !["N/D","Dado inválido"].includes(power.display) ? ` ${power.unit}` : ""}</strong>
    <small>{nominal != null ? `Nominal ${nominal} kW` : "Escala nominal N/D"}</small>
  </div>;
}

export function GeneratorVerticalCard({ row }: { row: FleetRow }) {
  const t = row.telemetry;
  const mode = metricView(t, "controller.mode");
  const activeAlarms = row.alarms.filter((alarm) => alarm.active);
  const fuel = metricOf(t, "fuel.level");
  const fuelPct = fuel && typeof fuel.value === "number" && fuel.unit === "%" && fuel.quality === "good" ? fuel.value : null;
  const engineRows = [
    { icon: <IconOilCan/>, label: "Oil Pressure", key: "engine.oil_pressure", digits: 1, pct: null },
    { icon: <IconThermometer/>, label: "Coolant Temp.", key: "engine.coolant_temperature", digits: 1, pct: null },
    { icon: <IconFuelPump/>, label: "Fuel Level", key: "fuel.level", digits: 0, pct: fuelPct },
    { icon: <IconBattery/>, label: "Battery Volt.", key: "battery.voltage", digits: 1, pct: null },
    { icon: <IconRunHours/>, label: "Run Hours", key: "engine.run_hours", digits: 1, pct: null },
    { icon: <IconBolt/>, label: "RPM", key: "engine.rpm", digits: 0, pct: null }
  ] as const;
  const volts = [
    ["L1-N Voltage", "mains.voltage_l1", "generator.voltage_l1"],
    ["L2-N Voltage", "mains.voltage_l2", "generator.voltage_l2"],
    ["L3-N Voltage", "mains.voltage_l3", "generator.voltage_l3"],
    ["L1-L2 Voltage", "mains.voltage_l1_l2", "generator.voltage_l1_l2"]
  ] as const;

  return <article className={`operator-generator-card communication-${t?.communication ?? "unknown"}`}>
    <header className="operator-card-header"><span className="operator-card-logo">G</span><div className="operator-card-identity"><h3>{row.generator.name}</h3><p>{row.generator.controller.manufacturer} {row.generator.controller.model}</p></div><span className={`operator-alarm ${activeAlarms.length ? "has-alarm" : ""}`} title="Alarmes ativos"><svg viewBox="0 0 24 24"><path d="M12 3 2.8 20h18.4L12 3Z"/><path d="M12 8.5v5.8m0 2.7h.01"/></svg><b>{activeAlarms.length || "—"}</b></span><Link to={`/generators/${encodeURIComponent(row.generator.id)}`} className="operator-home-link" aria-label="Abrir detalhes"><IconHouse size={14}/></Link></header>
    {t?.communication === "offline" ? <div className="operator-last-known">Última leitura conhecida · equipamento offline</div> : null}
    <section className="operator-mode"><span>MODE</span><div className="operator-mode-buttons">{["OFF","MAN","AUTO","TEST"].map((item) => <span key={item} className={mode.display.toUpperCase().startsWith(item) ? "is-active" : ""}>{item}</span>)}</div><strong>{mode.display}</strong></section>
    <section className="operator-flow"><div className="operator-section-heading"><h4>Power Flow</h4><span>READ ONLY</span></div><GeneratorEnergyFlow telemetry={t} compact/></section>
    <section className="operator-engine"><h4>Engine Status</h4>{engineRows.map((item) => { const view = metricView(t, item.key, { digits: item.digits }); return <EngineRow key={item.key} icon={item.icon} label={item.label} value={metricText(row, item.key, item.digits)} quality={view.quality} percent={item.pct}/>; })}</section>
    <section className="operator-gauge"><div className="operator-section-heading"><h4>Generator P</h4><span>POTÊNCIA ATIVA</span></div><PowerGauge row={row}/></section>
    <section className="operator-voltage-table"><h4>Mains / Generator</h4><div className="operator-voltage-head"><span/><span>Mains</span><span>Generator</span></div>{volts.map(([label,mainsKey,genKey]) => <div className="operator-voltage-row" key={label}><span>{label}</span><b>{metricText(row,mainsKey,0)}</b><strong>{metricText(row,genKey,0)}</strong></div>)}</section>
  </article>;
}

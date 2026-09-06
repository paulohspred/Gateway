import { Factory, RadioTower, Zap } from "lucide-react";
import type { Telemetry } from "../api/schemas";
import { metricOf, metricView, usableQuality } from "../lib/metric";
import { MetricValue } from "./MetricValue";
import { StatusBadge } from "./StatusBadge";

function Breaker({ label, metricKey, telemetry }: { label: string; metricKey: string; telemetry: Telemetry }) {
  const view = metricView(telemetry, metricKey, { booleanLabels: ["FECHADO", "ABERTO"] });
  if (!view.present) return null;
  const closed = view.raw === true && usableQuality(view.quality);
  return <div className="mimic-breaker"><span>{label}</span><div className={`breaker-symbol ${closed ? "closed" : "open"}`}><i/></div><StatusBadge tone={view.quality}>{view.display}</StatusBadge></div>;
}

export function ElectricalMimic({ telemetry }: { telemetry: Telemetry }) {
  const hasMains = ["mains.state", "mains.frequency", "mains.voltage_l1", "mains.voltage_l1_l2"].some((key) => Boolean(metricOf(telemetry, key)));
  const hasGen = ["generator.status", "generator.frequency", "generator.power_kw", "generator.voltage_l1"].some((key) => Boolean(metricOf(telemetry, key)));
  const ats = metricOf(telemetry, "ats.state");
  return <div className="mimic-grid">{hasMains ? <div className="mimic-source"><RadioTower/><strong>REDE</strong><MetricValue telemetry={telemetry} metricKey="mains.state"/><MetricValue telemetry={telemetry} metricKey="mains.frequency"/></div> : null}{hasMains ? <div className="mimic-line"><span/></div> : null}<Breaker label="MCB" metricKey="breaker.mcb" telemetry={telemetry}/>{hasMains ? <div className="mimic-line"><span/></div> : null}{ats ? <div className="mimic-node"><Zap/><strong>ATS</strong><MetricValue telemetry={telemetry} metricKey="ats.state"/></div> : null}{ats ? <div className="mimic-line"><span/></div> : null}<div className="mimic-node bus"><strong>BARRAMENTO</strong><span className="muted">estado não inferido</span></div><div className="mimic-line vertical"><span/></div>{hasGen ? <div className="mimic-source generator"><Factory/><strong>GERADOR</strong><MetricValue telemetry={telemetry} metricKey="generator.status"/><MetricValue telemetry={telemetry} metricKey="generator.power_kw"/></div> : null}<Breaker label="GCB" metricKey="breaker.gcb" telemetry={telemetry}/></div>;
}

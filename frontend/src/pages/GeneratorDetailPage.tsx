import { useQuery } from "@tanstack/react-query";
import { Activity, BatteryCharging, CircleGauge, Fuel, Info, RadioTower } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { api } from "../api/client";
import { ElectricalMimic } from "../components/ElectricalMimic";
import { MetricValue } from "../components/MetricValue";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

const motorMetrics = [
  ["RPM", "engine.rpm", 0], ["Estado", "engine.state", 0], ["Pressão do óleo", "engine.oil_pressure", 1], ["Temperatura do óleo", "engine.oil_temperature", 1],
  ["Nível do óleo", "engine.oil_level", 1], ["Temperatura coolant", "engine.coolant_temperature", 1], ["Nível coolant", "engine.coolant_level", 1], ["Horímetro", "engine.run_hours", 1], ["Partidas", "engine.starts", 0]
] as const;

const electricalMetrics = [
  ["V L1", "generator.voltage_l1"], ["V L2", "generator.voltage_l2"], ["V L3", "generator.voltage_l3"], ["V L1-L2", "generator.voltage_l1_l2"],
  ["V L2-L3", "generator.voltage_l2_l3"], ["V L3-L1", "generator.voltage_l3_l1"], ["A L1", "generator.current_l1"], ["A L2", "generator.current_l2"],
  ["A L3", "generator.current_l3"], ["Frequência", "generator.frequency"], ["Potência ativa", "generator.power_kw"], ["Potência aparente", "generator.power_kva"],
  ["Potência reativa", "generator.power_kvar"], ["Fator de potência", "generator.power_factor"], ["Carga", "generator.load_percent"], ["Energia", "generator.energy_kwh"]
] as const;

export function GeneratorDetailPage() {
  const { id = "" } = useParams();
  const generator = useQuery({ queryKey: ["generator", id], queryFn: () => api.getGenerator(id), enabled: Boolean(id) });
  const telemetry = useQuery({ queryKey: ["telemetry", id], queryFn: () => api.getTelemetry(id), enabled: Boolean(id), refetchInterval: 5_000 });
  const alarms = useQuery({ queryKey: ["alarms", id], queryFn: () => api.getAlarms(id), enabled: Boolean(id), refetchInterval: 10_000 });
  const events = useQuery({ queryKey: ["events", id], queryFn: () => api.getEvents(id), enabled: Boolean(id), refetchInterval: 10_000 });
  const health = useQuery({ queryKey: ["system-health"], queryFn: api.getSystemHealth, refetchInterval: 10_000 });

  const g = generator.data;
  const t = telemetry.data;
  if (generator.isLoading) return <><Topbar title="Gerador"/><div className="content-grid"><p className="loading">Carregando ativo…</p></div></>;
  if (!g) return <><Topbar title="Gerador não disponível"/><div className="content-grid"><div className="error-banner">{generator.error instanceof Error ? generator.error.message : "Gerador não encontrado"}</div></div></>;

  const activeAlarms = (alarms.data ?? []).filter((alarm) => alarm.active);
  return <>
    <Topbar title={g.name} subtitle={`${g.id} · Site ${g.siteId}`} onRefresh={() => { void telemetry.refetch(); void alarms.refetch(); void events.refetch(); void health.refetch(); }} refreshing={telemetry.isFetching || alarms.isFetching || events.isFetching}>
      <Link className="text-button" to="/generators">← Geradores</Link>
    </Topbar>
    <div className="content-grid detail-page">
      <section className="asset-strip">
        <div><span>Controladora</span><strong>{g.controller.manufacturer} {g.controller.model}</strong></div>
        <div><span>Comunicação</span><StatusBadge tone={t?.communication ?? "unknown"}>{(t?.communication ?? "unknown").toUpperCase()}</StatusBadge></div>
        <div><span>Modo</span><MetricValue telemetry={t} metricKey="controller.mode"/></div>
        <div><span>Status controlador</span><MetricValue telemetry={t} metricKey="controller.status"/></div>
        <div><span>Snapshot</span><strong>{formatDateTime(t?.capturedAt)}</strong></div>
      </section>

      <section className="quality-strip">
        <div><RadioTower/><span>Provider</span><StatusBadge tone={health.data?.provider.status ?? "unavailable"}>{(health.data?.provider.status ?? "unavailable").toUpperCase()}</StatusBadge></div>
        <div><Info/><span>Qualidade é mostrada por métrica. Ausência permanece N/D e zero real permanece zero.</span></div>
      </section>

      {t ? <Panel title="Sinótico elétrico"><ElectricalMimic telemetry={t}/></Panel> : <Panel title="Sinótico elétrico"><p className="empty">Telemetria indisponível. Nenhum estado elétrico será inferido.</p></Panel>}

      <div className="two-col">
        <Panel title="Motor / ECU">
          <div className="metric-grid">{motorMetrics.map(([label, key, digits]) => <div className="metric-cell" key={key}><span>{label}</span><MetricValue telemetry={t} metricKey={key} digits={digits}/></div>)}</div>
          <p className="panel-note">ECU/J1939 adicional só aparecerá após MetricKeys e profiles homologados.</p>
        </Panel>
        <Panel title="Elétrica">
          <div className="metric-grid electrical">{electricalMetrics.map(([label, key]) => <div className="metric-cell" key={key}><span>{label}</span><MetricValue telemetry={t} metricKey={key}/></div>)}</div>
        </Panel>
      </div>

      <div className="two-col">
        <Panel title="Combustível" action={<Fuel/>}>
          <div className="metric-grid"><div className="metric-cell"><span>Nível</span><MetricValue telemetry={t} metricKey="fuel.level"/></div><div className="metric-cell"><span>Consumo instantâneo</span><MetricValue telemetry={t} metricKey="fuel.consumption_rate"/></div><div className="metric-cell"><span>Consumo total</span><MetricValue telemetry={t} metricKey="fuel.total_consumption"/></div></div>
        </Panel>
        <Panel title="DC / Bateria" action={<BatteryCharging/>}>
          <div className="metric-grid"><div className="metric-cell"><span>Tensão bateria</span><MetricValue telemetry={t} metricKey="battery.voltage"/></div><div className="metric-cell"><span>Corrente bateria</span><MetricValue telemetry={t} metricKey="battery.current"/></div><div className="metric-cell"><span>Tensão carregador</span><MetricValue telemetry={t} metricKey="battery.charger_voltage"/></div><div className="metric-cell"><span>Corrente carregador</span><MetricValue telemetry={t} metricKey="battery.charger_current"/></div></div>
        </Panel>
      </div>

      <Panel title={`Alarmes ativos (${activeAlarms.length})`} action={<CircleGauge/>}>
        <div className="table-wrap"><table className="data-table"><thead><tr><th>Severidade</th><th>Código</th><th>Mensagem</th><th>Desde</th></tr></thead><tbody>{activeAlarms.length ? activeAlarms.map((alarm) => <tr key={alarm.id}><td><StatusBadge tone={alarm.severity}>{alarm.severity.toUpperCase()}</StatusBadge></td><td>{alarm.code}</td><td>{alarm.message}</td><td>{formatDateTime(alarm.raisedAt)}</td></tr>) : <tr><td colSpan={4} className="empty-cell">Nenhum alarme ativo retornado.</td></tr>}</tbody></table></div>
      </Panel>

      <Panel title="Eventos recentes" action={<Activity/>}>
        <div className="table-wrap"><table className="data-table"><thead><tr><th>Hora</th><th>Tipo</th><th>Mensagem</th></tr></thead><tbody>{(events.data ?? []).slice(0, 12).map((event) => <tr key={event.id}><td>{formatDateTime(event.occurredAt)}</td><td>{event.type}</td><td>{event.message}</td></tr>)}</tbody></table></div>
      </Panel>

      <details className="technical-details"><summary>Detalhes técnicos e qualidade</summary><div className="technical-grid"><div><span>Firmware</span><strong>{g.controller.firmware || "N/D"}</strong></div><div><span>Hardware</span><strong>{g.controller.hardwareVersion || "N/D"}</strong></div><div><span>Serial</span><strong>{g.controller.serialNumber || "N/D"}</strong></div><div><span>Potência nominal</span><strong>{g.spec?.ratedPowerKw ?? "N/D"}{g.spec?.ratedPowerKw !== undefined ? " kW" : ""}</strong></div><div><span>Tensão nominal</span><strong>{g.spec?.nominalVoltage ?? "N/D"}{g.spec?.nominalVoltage !== undefined ? " V" : ""}</strong></div><div><span>Frequência nominal</span><strong>{g.spec?.nominalFrequency ?? "N/D"}{g.spec?.nominalFrequency !== undefined ? " Hz" : ""}</strong></div></div>{t ? <div className="raw-metrics">{Object.entries(t.metrics).sort(([a], [b]) => a.localeCompare(b)).map(([key, metric]) => <div key={key}><code>{key}</code><span>{String(metric.value)} {metric.unit ?? ""}</span><StatusBadge tone={metric.quality}>{metric.quality}</StatusBadge><span>{formatDateTime(metric.observedAt)}</span></div>)}</div> : null}</details>
    </div>
  </>;
}

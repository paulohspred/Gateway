import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { controlApi } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

const stages = ["IDENTITY", "TRANSPORT", "CONTROLLER", "PROFILE", "RAPID_PLAN", "RAPID_APPLY", "TELEMETRY_VALIDATION", "EVIDENCE"] as const;
const descriptions: Record<string, string> = {
  IDENTITY: "Identidade e nominais do ativo",
  TRANSPORT: "Caminho físico/lógico e preflight",
  CONTROLLER: "Controladora identificada",
  PROFILE: "Profile/Controller Pack e qualificação",
  RAPID_PLAN: "Plano Rapid versionado, hash/diff e rollback",
  RAPID_APPLY: "Resultado de aplicação externa aprovada. O browser não escreve Rapid.",
  TELEMETRY_VALIDATION: "Required metrics recuperáveis, tipos/unidades/quality válidos",
  EVIDENCE: "Pacote final de evidências e aprovação"
};

function tone(status: string) {
  if (status === "PASS" || status === "COMMISSIONED") return "healthy";
  if (status === "FAIL") return "critical";
  if (status === "BLOCKED" || status === "DRAFT") return "warning";
  if (status === "RETIRED" || status === "SUPERSEDED") return "offline";
  return "unknown";
}

export function CommissioningDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const auth = useAuth();
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: ["commissioning", id], queryFn: () => controlApi.commissioning(id), refetchInterval: 10_000 });
  const [evidence, setEvidence] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [changeReason, setChangeReason] = useState("");
  const [preflight, setPreflight] = useState<Awaited<ReturnType<typeof controlApi.preflightCommissioning>> | null>(null);
  const [validation, setValidation] = useState<Awaited<ReturnType<typeof controlApi.validateTelemetry>> | null>(null);

  const invalidate = () => {
    void query.refetch();
    void queryClient.invalidateQueries({ queryKey: ["commissionings"] });
  };

  const gateMutation = useMutation({
    mutationFn: ({ stage, status }: { stage: string; status: string }) => controlApi.setGate(auth.csrf, id, stage, status, evidence[stage] ?? ""),
    onSuccess: () => { setMessage("Gate atualizado e auditado."); invalidate(); },
    onError: (error) => setMessage(error instanceof Error ? error.message : "Falha ao atualizar gate")
  });

  if (!query.data) {
    return <><Topbar title="Commissioning"/><div className="content-grid"><p className="loading">{query.isLoading ? "Carregando…" : query.error instanceof Error ? query.error.message : "N/D"}</p></div></>;
  }

  const commissioning = query.data;
  const allPass = stages.every((stage) => commissioning.gates[stage]?.status === "PASS");
  const mutable = commissioning.lifecycle === "DRAFT";

  const runPreflight = async () => {
    try {
      const result = await controlApi.preflightCommissioning(auth.csrf, id);
      setPreflight(result);
      setMessage(result.pass ? "Preflight automatizado PASS." : "Preflight encontrou pendências.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Falha no preflight");
    }
  };

  const runTelemetry = async () => {
    try {
      const result = await controlApi.validateTelemetry(auth.csrf, id);
      setValidation(result);
      setMessage(result.pass ? "Telemetria required PASS e gate atualizado automaticamente." : "Telemetria required falhou.");
      invalidate();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Falha na validação de telemetria");
    }
  };

  const exportEvidence = async () => {
    try {
      const snapshot = await controlApi.commissioningEvidence(id);
      const blob = new Blob([JSON.stringify(snapshot, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `commissioning-${commissioning.tag}-rev${commissioning.revision}-evidence.json`;
      link.click();
      URL.revokeObjectURL(url);
      setMessage("Pacote factual de evidências exportado.");
    } catch (error) { setMessage(error instanceof Error ? error.message : "Falha ao exportar evidências"); }
  };

  const createChange = async () => {
    if (!changeReason.trim()) { setMessage("Informe o motivo da alteração."); return; }
    try {
      const child = await controlApi.createChangeCommissioning(auth.csrf, id, changeReason);
      navigate(`/engineering/commissioning/${child.id}`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Falha ao criar Change Commissioning");
    }
  };

  return <>
    <Topbar title={`${commissioning.tag} · ${commissioning.name}`} subtitle={`${commissioning.lifecycle} · revisão ${commissioning.revision} · ${commissioning.stage}`} onRefresh={() => void query.refetch()} refreshing={query.isFetching}>
      <Link className="text-button" to="/engineering/commissioning">← Fila</Link>
      <button className="text-button" onClick={() => void exportEvidence()}>Exportar evidências</button>
      {auth.hasPermission("commissioning.write") && mutable ? <Link className="primary-button compact" to={`/engineering/commissioning/${commissioning.id}/edit`}>Editar dados</Link> : null}
    </Topbar>
    <div className="content-grid commissioning-detail">
      {message ? <div className="info-banner">{message}</div> : null}
      <section className="asset-strip">
        <div><span>Lifecycle</span><StatusBadge tone={tone(commissioning.lifecycle)}>{commissioning.lifecycle}</StatusBadge></div>
        <div><span>Etapa atual</span><strong>{commissioning.stage}</strong></div>
        <div><span>Site</span><strong>{commissioning.siteId}</strong></div>
        <div><span>RC Monitor ID</span><strong>{commissioning.monitorGeneratorId || "N/D"}</strong></div>
        <div><span>Revisão</span><strong>{commissioning.revision}</strong></div>
        <div><span>Atualizado</span><strong>{formatDateTime(commissioning.updatedAt)}</strong></div>
      </section>

      {commissioning.parentCommissioningId ? <div className="info-banner">Change Commissioning da revisão <Link to={`/engineering/commissioning/${commissioning.parentCommissioningId}`}>{commissioning.parentCommissioningId}</Link> · motivo: {commissioning.changeReason || "N/D"}</div> : null}

      <div className="two-col">
        <Panel title="Identidade"><div className="technical-grid"><div><span>Tag</span><strong>{commissioning.tag}</strong></div><div><span>Nome</span><strong>{commissioning.name}</strong></div>{Object.entries(commissioning.asset).map(([key, value]) => <div key={key}><span>{key}</span><strong>{String(value ?? "N/D")}</strong></div>)}</div></Panel>
        <Panel title="Controladora / ECU / Transporte"><div className="technical-grid">{Object.entries(commissioning.controller).map(([key, value]) => <div key={`c-${key}`}><span>controller.{key}</span><strong>{String(value || "N/D")}</strong></div>)}{Object.entries(commissioning.ecu ?? {}).map(([key, value]) => <div key={`e-${key}`}><span>ecu.{key}</span><strong>{typeof value === "boolean" ? (value ? "SIM" : "NÃO") : String(value || "N/D")}</strong></div>)}{Object.entries(commissioning.transport).map(([key, value]) => <div key={`t-${key}`}><span>transport.{key}</span><strong>{String(value || "N/D")}</strong></div>)}</div></Panel>
      </div>

      {auth.hasPermission("commissioning.write") && mutable ? <Panel title="Validações automatizadas">
        <div className="validation-actions"><button className="primary-button" onClick={() => void runPreflight()}>Executar preflight</button><button className="primary-button" disabled={!commissioning.monitorGeneratorId} onClick={() => void runTelemetry()}>Validar telemetria required</button></div>
        {preflight ? <div className="validation-grid">{preflight.checks.map((check) => <div key={check.id}><StatusBadge tone={check.status === "PASS" ? "healthy" : "critical"}>{check.status}</StatusBadge><code>{check.id}</code><span>{check.message}</span></div>)}</div> : null}
        {validation ? <div className="validation-grid"><div><StatusBadge tone={validation.pass ? "healthy" : "critical"}>{validation.pass ? "PASS" : "FAIL"}</StatusBadge><code>communication</code><span>{validation.communication}</span></div>{Object.entries(validation.requiredMetrics).map(([key, value]) => <div key={key}><StatusBadge tone={value === "GOOD" ? "healthy" : "critical"}>{value}</StatusBadge><code>{key}</code><span>required metric</span></div>)}</div> : null}
      </Panel> : null}

      <Panel title="Gates de commissioning"><div className="gate-list">{stages.map((stage) => {
        const gate = commissioning.gates[stage];
        return <article key={stage} className={`gate-card gate-${gate?.status?.toLowerCase() ?? "pending"}`}>
          <header><div><code>{stage}</code><p>{descriptions[stage]}</p></div><StatusBadge tone={tone(gate?.status ?? "PENDING")}>{gate?.status ?? "PENDING"}</StatusBadge></header>
          {gate?.evidence ? <blockquote>{gate.evidence}</blockquote> : null}
          {stage === "TELEMETRY_VALIDATION" ? <p className="panel-note">Gate automático: somente a validação de telemetria required pode atualizar este resultado.</p> : auth.hasPermission("commissioning.write") && mutable ? <><textarea value={evidence[stage] ?? ""} onChange={(event) => setEvidence((current) => ({ ...current, [stage]: event.target.value }))} placeholder="Evidência objetiva obrigatória para PASS; referência de teste/hash/log…"/><div className="gate-actions"><button disabled={!evidence[stage]?.trim()} onClick={() => gateMutation.mutate({ stage, status: "PASS" })}>PASS</button><button onClick={() => gateMutation.mutate({ stage, status: "FAIL" })}>FAIL</button><button onClick={() => gateMutation.mutate({ stage, status: "BLOCKED" })}>BLOCKED</button></div></> : null}
        </article>;
      })}</div></Panel>

      <Panel title="Promoção / lifecycle"><div className="promotion-box"><p>Somente `COMMISSIONED` entra em Operação. A promoção exige oito gates PASS, vínculo ao RC Monitor e site coerente.</p><div>
        {auth.hasPermission("commissioning.promote") && mutable ? <button className="primary-button" disabled={!allPass || !commissioning.monitorGeneratorId} onClick={async () => { try { await controlApi.promoteCommissioning(auth.csrf, id); setMessage("Revisão promovida a COMMISSIONED."); invalidate(); } catch (error) { setMessage(error instanceof Error ? error.message : "Falha"); } }}>Comissionar gerador</button> : null}
        {auth.hasPermission("commissioning.write") && commissioning.lifecycle === "COMMISSIONED" ? <button onClick={async () => { await controlApi.suspendCommissioning(auth.csrf, id); invalidate(); }}>Suspender</button> : null}
        {auth.hasPermission("commissioning.write") && !["RETIRED", "SUPERSEDED"].includes(commissioning.lifecycle) ? <button className="danger-button" onClick={async () => { if (window.confirm("Retirar este ativo? O histórico será preservado.")) { await controlApi.retireCommissioning(auth.csrf, id); invalidate(); } }}>Retirar</button> : null}
      </div></div>
      {auth.hasPermission("commissioning.write") && (commissioning.lifecycle === "COMMISSIONED" || commissioning.lifecycle === "SUSPENDED") ? <div className="change-box"><label>Motivo da alteração<input value={changeReason} onChange={(event) => setChangeReason(event.target.value)} placeholder="Ex.: atualização de firmware/profile, substituição de controladora…"/></label><button className="primary-button" onClick={() => void createChange()}>Criar Change Commissioning</button></div> : null}
      </Panel>
    </div>
  </>;
}

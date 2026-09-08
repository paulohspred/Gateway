import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { controlApi } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";

const steps = ["Ativo", "Controladora / ECU", "Transporte", "Rapid plan", "RC Monitor", "Revisão"];
type FormState = {
  tag:string; name:string; siteId:string; ratedPowerKw:string; nominalVoltage:string; nominalFrequency:string; phaseCount:string; nominalRpm:string;
  manufacturer:string; model:string; firmware:string; hardware:string; serialNumber:string; profileId:string; profileVersion:string;
  ecuManufacturer:string; ecuModel:string; ecuSerial:string; ecuProtocol:string; ecuJ1939:boolean;
  transportKind:string; gateway:string; host:string; port:string; deviceAddress:string; notes:string;
  rapidPlanHash:string; rapidPlanVersion:string; monitorGeneratorId:string;
};
const initial: FormState = { tag:"",name:"",siteId:"",ratedPowerKw:"",nominalVoltage:"",nominalFrequency:"60",phaseCount:"3",nominalRpm:"1800",manufacturer:"",model:"",firmware:"",hardware:"",serialNumber:"",profileId:"",profileVersion:"",ecuManufacturer:"",ecuModel:"",ecuSerial:"",ecuProtocol:"",ecuJ1939:false,transportKind:"TCP/IP",gateway:"",host:"",port:"",deviceAddress:"",notes:"",rapidPlanHash:"",rapidPlanVersion:"",monitorGeneratorId:"" };

export function NewCommissioningPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const sites = useQuery({ queryKey:["admin-sites"], queryFn:controlApi.sites });
  const profiles = useQuery({ queryKey:["profiles"], queryFn:controlApi.profiles });
  const profileStates = useQuery({ queryKey:["profile-states"], queryFn:controlApi.profileStates });
  const monitorGenerators = useQuery({ queryKey:["engineering-monitor-generators"], queryFn:controlApi.monitorGenerators });
  const [step,setStep] = useState(0);
  const [error,setError] = useState("");
  const [busy,setBusy] = useState(false);
  const [form,setForm] = useState<FormState>(initial);
  const patch = <K extends keyof FormState>(key:K,value:FormState[K]) => setForm((current)=>({...current,[key]:value}));
  const selectedProfile = useMemo(()=>profiles.data?.profiles.find((p)=>p.id===form.profileId),[profiles.data,form.profileId]);
  const selectedProfileState = profileStates.data?.find((state)=>state.profileId===form.profileId);
  const candidates = (monitorGenerators.data ?? []).filter((g)=>!form.siteId || g.siteId===form.siteId);

  const chooseProfile = (id:string) => {
    const profile = profiles.data?.profiles.find((candidate)=>candidate.id===id);
    setForm((current)=>({...current,profileId:id,manufacturer:profile?.manufacturer??current.manufacturer,model:profile?.model??current.model}));
  };
  const toNumber = (value:string) => value.trim() ? Number(value) : undefined;
  const create = async () => {
    setBusy(true); setError("");
    try {
      const commissioning = await controlApi.createCommissioning(auth.csrf,{
        tag:form.tag,name:form.name,siteId:form.siteId,
        asset:{ratedPowerKw:toNumber(form.ratedPowerKw),nominalVoltage:toNumber(form.nominalVoltage),nominalFrequency:toNumber(form.nominalFrequency),phaseCount:form.phaseCount?Number(form.phaseCount):undefined,nominalRpm:toNumber(form.nominalRpm)},
        controller:{manufacturer:form.manufacturer,model:form.model,firmware:form.firmware,hardware:form.hardware,serialNumber:form.serialNumber,profileId:form.profileId,profileVersion:form.profileVersion,profileStatus:selectedProfileState?.status??"DRAFT"},
        ecu:{manufacturer:form.ecuManufacturer,model:form.ecuModel,serialNumber:form.ecuSerial,protocol:form.ecuProtocol,j1939:form.ecuJ1939},
        transport:{kind:form.transportKind,gateway:form.gateway,host:form.host,port:form.port?Number(form.port):0,deviceAddress:form.deviceAddress,notes:form.notes},
        rapidPlanHash:form.rapidPlanHash,rapidPlanVersion:form.rapidPlanVersion,monitorGeneratorId:form.monitorGeneratorId
      });
      navigate(`/engineering/commissioning/${commissioning.id}`);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Falha ao criar commissioning"); }
    finally { setBusy(false); }
  };
  const canNext = step===0 ? Boolean(form.tag&&form.name&&form.siteId) : step===1 ? Boolean(form.manufacturer&&form.model) : step===2 ? Boolean(form.transportKind) : true;

  return <>
    <Topbar title="Adicionar gerador" subtitle="Cria um commissioning DRAFT; o ativo só entra na Operação após todos os gates PASS"/>
    <div className="content-grid">
      <div className="wizard-steps">{steps.map((label,index)=><div key={label} className={index===step?"active":index<step?"done":""}><b>{index+1}</b><span>{label}</span></div>)}</div>
      {error ? <div className="error-banner">{error}</div> : null}
      <Panel title={steps[step] ?? "Etapa"}>
        {step===0 ? <div className="wizard-form">
          <label>Tag<input value={form.tag} onChange={(e)=>patch("tag",e.target.value)} placeholder="GEN-001" required/></label>
          <label>Nome<input value={form.name} onChange={(e)=>patch("name",e.target.value)} required/></label>
          <label>Site<select value={form.siteId} onChange={(e)=>patch("siteId",e.target.value)}><option value="">Selecione…</option>{sites.data?.filter((site)=>site.active).map((site)=><option key={site.id} value={site.id}>{site.code} · {site.name}</option>)}</select></label>
          <label>Potência nominal (kW)<input type="number" min="0" value={form.ratedPowerKw} onChange={(e)=>patch("ratedPowerKw",e.target.value)}/></label>
          <label>Tensão nominal (V)<input type="number" min="0" value={form.nominalVoltage} onChange={(e)=>patch("nominalVoltage",e.target.value)}/></label>
          <label>Frequência nominal (Hz)<input type="number" value={form.nominalFrequency} onChange={(e)=>patch("nominalFrequency",e.target.value)}/></label>
          <label>Fases<select value={form.phaseCount} onChange={(e)=>patch("phaseCount",e.target.value)}><option value="1">1</option><option value="3">3</option></select></label>
          <label>RPM nominal<input type="number" value={form.nominalRpm} onChange={(e)=>patch("nominalRpm",e.target.value)}/></label>
        </div> : null}

        {step===1 ? <div className="wizard-form">
          <label>Controller Profile<select value={form.profileId} onChange={(e)=>chooseProfile(e.target.value)}><option value="">Selecionar catálogo ou informar manualmente…</option>{profiles.data?.profiles.map((profile)=><option key={profile.id} value={profile.id}>{profile.manufacturer} · {profile.model}</option>)}</select></label>
          <label>Lifecycle do profile<input value={selectedProfileState?.status??"DRAFT"} readOnly/></label>
          <label>Fabricante controladora<input value={form.manufacturer} onChange={(e)=>patch("manufacturer",e.target.value)} required/></label>
          <label>Modelo controladora<input value={form.model} onChange={(e)=>patch("model",e.target.value)} required/></label>
          <label>Firmware<input value={form.firmware} onChange={(e)=>patch("firmware",e.target.value)}/></label><label>Hardware<input value={form.hardware} onChange={(e)=>patch("hardware",e.target.value)}/></label><label>Serial<input value={form.serialNumber} onChange={(e)=>patch("serialNumber",e.target.value)}/></label><label>Profile version<input value={form.profileVersion} onChange={(e)=>patch("profileVersion",e.target.value)}/></label>
          <label>Fabricante ECU<input value={form.ecuManufacturer} onChange={(e)=>patch("ecuManufacturer",e.target.value)}/></label><label>Modelo ECU<input value={form.ecuModel} onChange={(e)=>patch("ecuModel",e.target.value)}/></label><label>Serial ECU<input value={form.ecuSerial} onChange={(e)=>patch("ecuSerial",e.target.value)}/></label><label>Protocolo ECU<input value={form.ecuProtocol} onChange={(e)=>patch("ecuProtocol",e.target.value)} placeholder="J1939 / CAN / N/D"/></label><label className="check-row"><input type="checkbox" checked={form.ecuJ1939} onChange={(e)=>patch("ecuJ1939",e.target.checked)}/> J1939 disponível</label>
          {selectedProfile ? <div className="wizard-note span-2">{selectedProfile.metrics.length} métricas e {selectedProfile.alarms.length} alarmes declarados no catálogo. Lifecycle {selectedProfileState?.status??"DRAFT"}; isso não implica HIL.</div> : null}
        </div> : null}

        {step===2 ? <div className="wizard-form"><label>Meio<select value={form.transportKind} onChange={(e)=>patch("transportKind",e.target.value)}><option>TCP/IP</option><option>Serial / RS485</option><option>VPN</option><option>Modem</option><option>Gateway local</option><option>CAN/J1939</option></select></label><label>Gateway/rota<input value={form.gateway} onChange={(e)=>patch("gateway",e.target.value)}/></label><label>Host<input value={form.host} onChange={(e)=>patch("host",e.target.value)}/></label><label>Porta<input type="number" min="0" max="65535" value={form.port} onChange={(e)=>patch("port",e.target.value)}/></label><label>Device / Unit ID<input value={form.deviceAddress} onChange={(e)=>patch("deviceAddress",e.target.value)}/></label><label className="span-2">Notas<input value={form.notes} onChange={(e)=>patch("notes",e.target.value)}/></label><div className="wizard-note span-2">Esta etapa descreve transporte. O Gateway permanece BRIDGE FIRST; mapas físicos ficam no Rapid/Profile.</div></div> : null}

        {step===3 ? <div className="wizard-form"><label>Rapid plan version<input value={form.rapidPlanVersion} onChange={(e)=>patch("rapidPlanVersion",e.target.value)} placeholder="preenchido quando o plano existir"/></label><label>Rapid plan SHA-256<input value={form.rapidPlanHash} onChange={(e)=>patch("rapidPlanHash",e.target.value)} placeholder="64 hex do plano versionado"/></label><div className="wizard-note span-2">O navegador não escreve XML/DAT. RAPID_APPLY registra somente aplicação externa aprovada, diff/hash e evidência.</div></div> : null}

        {step===4 ? <div className="wizard-form"><label className="span-2">RC Monitor generatorId<input list="monitor-generator-candidates" value={form.monitorGeneratorId} onChange={(e)=>patch("monitorGeneratorId",e.target.value)} placeholder="Pode ser vinculado agora ou antes da promoção"/><datalist id="monitor-generator-candidates">{candidates.map((generator)=><option key={generator.id} value={generator.id}>{generator.name} · {generator.siteId}</option>)}</datalist></label><div className="wizard-note span-2">Engenharia pode enxergar candidatos configurados no RC Monitor antes do commissioning. Operação continua vendo somente revisões COMMISSIONED.</div></div> : null}

        {step===5 ? <div className="review-grid"><div><span>Ativo</span><strong>{form.tag} · {form.name}</strong></div><div><span>Site</span><strong>{form.siteId}</strong></div><div><span>Controladora</span><strong>{form.manufacturer} {form.model}</strong></div><div><span>ECU</span><strong>{form.ecuManufacturer||form.ecuModel ? `${form.ecuManufacturer} ${form.ecuModel}` : "N/D"}</strong></div><div><span>Profile</span><strong>{form.profileId||"N/D"} · {selectedProfileState?.status??"DRAFT"}</strong></div><div><span>Transporte</span><strong>{form.transportKind} {form.host}{form.port?`:${form.port}`:""}</strong></div><div><span>Rapid plan</span><strong>{form.rapidPlanVersion||"PENDENTE"}</strong></div><div><span>RC Monitor</span><strong>{form.monitorGeneratorId||"PENDENTE"}</strong></div><div className="span-2"><span>Resultado desta ação</span><strong>DRAFT / IDENTITY — fora da frota operacional</strong></div></div> : null}

        <div className="wizard-actions"><button disabled={step===0} onClick={()=>setStep((current)=>current-1)}>Voltar</button>{step<steps.length-1 ? <button className="primary-button" disabled={!canNext} onClick={()=>setStep((current)=>current+1)}>Continuar</button> : <button className="primary-button" disabled={busy} onClick={()=>void create()}>{busy?"Criando…":"Criar commissioning"}</button>}</div>
      </Panel>
    </div>
  </>;
}

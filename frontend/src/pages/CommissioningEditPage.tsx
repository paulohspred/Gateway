import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { controlApi } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { Panel } from "../components/Panel";
import { Topbar } from "../components/Topbar";

type FormState = {
  tag:string; name:string; siteId:string; ratedPowerKw:string; nominalVoltage:string; nominalFrequency:string; phaseCount:string; nominalRpm:string;
  manufacturer:string; model:string; firmware:string; hardware:string; serialNumber:string; profileId:string; profileVersion:string;
  ecuManufacturer:string; ecuModel:string; ecuSerial:string; ecuProtocol:string; ecuJ1939:boolean;
  transportKind:string; gateway:string; host:string; port:string; deviceAddress:string; notes:string;
  rapidPlanHash:string; rapidPlanVersion:string; monitorGeneratorId:string; changeReason:string;
};
const empty: FormState = { tag:"",name:"",siteId:"",ratedPowerKw:"",nominalVoltage:"",nominalFrequency:"",phaseCount:"",nominalRpm:"",manufacturer:"",model:"",firmware:"",hardware:"",serialNumber:"",profileId:"",profileVersion:"",ecuManufacturer:"",ecuModel:"",ecuSerial:"",ecuProtocol:"",ecuJ1939:false,transportKind:"",gateway:"",host:"",port:"",deviceAddress:"",notes:"",rapidPlanHash:"",rapidPlanVersion:"",monitorGeneratorId:"",changeReason:"" };
const text = (value:unknown) => value == null ? "" : String(value);

export function CommissioningEditPage() {
  const { id = "" } = useParams();
  const auth = useAuth();
  const navigate = useNavigate();
  const query = useQuery({ queryKey:["commissioning",id], queryFn:()=>controlApi.commissioning(id) });
  const sites = useQuery({ queryKey:["admin-sites"], queryFn:controlApi.sites });
  const profiles = useQuery({ queryKey:["profiles"], queryFn:controlApi.profiles });
  const profileStates = useQuery({ queryKey:["profile-states"], queryFn:controlApi.profileStates });
  const monitorGenerators = useQuery({ queryKey:["engineering-monitor-generators"], queryFn:controlApi.monitorGenerators });
  const [form,setForm] = useState<FormState>(empty);
  const [error,setError] = useState("");
  const [busy,setBusy] = useState(false);

  useEffect(()=>{
    const commissioning = query.data; if(!commissioning) return;
    const asset=commissioning.asset as Record<string,unknown>; const controller=commissioning.controller as Record<string,unknown>; const ecu=commissioning.ecu as Record<string,unknown>; const transport=commissioning.transport as Record<string,unknown>;
    setForm({tag:commissioning.tag,name:commissioning.name,siteId:commissioning.siteId,ratedPowerKw:text(asset.ratedPowerKw),nominalVoltage:text(asset.nominalVoltage),nominalFrequency:text(asset.nominalFrequency),phaseCount:text(asset.phaseCount),nominalRpm:text(asset.nominalRpm),manufacturer:text(controller.manufacturer),model:text(controller.model),firmware:text(controller.firmware),hardware:text(controller.hardware),serialNumber:text(controller.serialNumber),profileId:text(controller.profileId),profileVersion:text(controller.profileVersion),ecuManufacturer:text(ecu.manufacturer),ecuModel:text(ecu.model),ecuSerial:text(ecu.serialNumber),ecuProtocol:text(ecu.protocol),ecuJ1939:Boolean(ecu.j1939),transportKind:text(transport.kind),gateway:text(transport.gateway),host:text(transport.host),port:text(transport.port),deviceAddress:text(transport.deviceAddress),notes:text(transport.notes),rapidPlanHash:commissioning.rapidPlanHash??"",rapidPlanVersion:commissioning.rapidPlanVersion??"",monitorGeneratorId:commissioning.monitorGeneratorId??"",changeReason:commissioning.changeReason??""});
  },[query.data]);

  const patch = <K extends keyof FormState>(key:K,value:FormState[K]) => setForm((current)=>({...current,[key]:value}));
  const selectedState = profileStates.data?.find((state)=>state.profileId===form.profileId);
  const candidates = (monitorGenerators.data ?? []).filter((g)=>!form.siteId || g.siteId===form.siteId);
  if(query.data && query.data.lifecycle !== "DRAFT") return <><Topbar title="Editar commissioning"/><div className="content-grid"><div className="warning-banner">Esta revisão não pode ser editada. Crie um Change Commissioning.</div><Link to={`/engineering/commissioning/${id}`}>Voltar</Link></div></>;

  const save = async () => {
    setBusy(true); setError("");
    try {
      const number=(value:string)=>value.trim()?Number(value):undefined;
      await controlApi.updateCommissioning(auth.csrf,id,{
        tag:form.tag,name:form.name,siteId:form.siteId,
        asset:{ratedPowerKw:number(form.ratedPowerKw),nominalVoltage:number(form.nominalVoltage),nominalFrequency:number(form.nominalFrequency),phaseCount:form.phaseCount?Number(form.phaseCount):undefined,nominalRpm:number(form.nominalRpm)},
        controller:{manufacturer:form.manufacturer,model:form.model,firmware:form.firmware,hardware:form.hardware,serialNumber:form.serialNumber,profileId:form.profileId,profileVersion:form.profileVersion,profileStatus:selectedState?.status??"DRAFT"},
        ecu:{manufacturer:form.ecuManufacturer,model:form.ecuModel,serialNumber:form.ecuSerial,protocol:form.ecuProtocol,j1939:form.ecuJ1939},
        transport:{kind:form.transportKind,gateway:form.gateway,host:form.host,port:form.port?Number(form.port):0,deviceAddress:form.deviceAddress,notes:form.notes},
        rapidPlanHash:form.rapidPlanHash,rapidPlanVersion:form.rapidPlanVersion,monitorGeneratorId:form.monitorGeneratorId,changeReason:form.changeReason
      });
      navigate(`/engineering/commissioning/${id}`);
    } catch(cause) { setError(cause instanceof Error?cause.message:"Falha ao salvar"); }
    finally { setBusy(false); }
  };

  return <>
    <Topbar title="Editar commissioning" subtitle={query.data?`${query.data.tag} · revisão ${query.data.revision}`:id}><Link className="text-button" to={`/engineering/commissioning/${id}`}>← Cancelar</Link></Topbar>
    <div className="content-grid">
      {error?<div className="error-banner">{error}</div>:null}
      <Panel title="Ativo / Controladora / ECU"><div className="admin-form-grid">
        <label>Tag<input value={form.tag} onChange={(e)=>patch("tag",e.target.value)}/></label><label>Nome<input value={form.name} onChange={(e)=>patch("name",e.target.value)}/></label><label>Site<select value={form.siteId} onChange={(e)=>patch("siteId",e.target.value)}>{sites.data?.filter((site)=>site.active).map((site)=><option key={site.id} value={site.id}>{site.code} · {site.name}</option>)}</select></label>
        <label>Potência kW<input type="number" value={form.ratedPowerKw} onChange={(e)=>patch("ratedPowerKw",e.target.value)}/></label><label>Tensão V<input type="number" value={form.nominalVoltage} onChange={(e)=>patch("nominalVoltage",e.target.value)}/></label><label>Frequência Hz<input type="number" value={form.nominalFrequency} onChange={(e)=>patch("nominalFrequency",e.target.value)}/></label><label>Fases<input type="number" value={form.phaseCount} onChange={(e)=>patch("phaseCount",e.target.value)}/></label><label>RPM nominal<input type="number" value={form.nominalRpm} onChange={(e)=>patch("nominalRpm",e.target.value)}/></label>
        <label>Profile<select value={form.profileId} onChange={(e)=>{const profileId=e.target.value;const profile=profiles.data?.profiles.find((candidate)=>candidate.id===profileId);setForm((current)=>({...current,profileId,manufacturer:profile?.manufacturer??current.manufacturer,model:profile?.model??current.model}))}}><option value="">Selecione…</option>{profiles.data?.profiles.map((profile)=><option key={profile.id} value={profile.id}>{profile.manufacturer} · {profile.model}</option>)}</select></label><label>Profile lifecycle<input value={selectedState?.status??"DRAFT"} readOnly/></label><label>Fabricante<input value={form.manufacturer} onChange={(e)=>patch("manufacturer",e.target.value)}/></label><label>Modelo<input value={form.model} onChange={(e)=>patch("model",e.target.value)}/></label><label>Firmware<input value={form.firmware} onChange={(e)=>patch("firmware",e.target.value)}/></label><label>Hardware<input value={form.hardware} onChange={(e)=>patch("hardware",e.target.value)}/></label><label>Serial<input value={form.serialNumber} onChange={(e)=>patch("serialNumber",e.target.value)}/></label><label>Profile version<input value={form.profileVersion} onChange={(e)=>patch("profileVersion",e.target.value)}/></label>
        <label>Fabricante ECU<input value={form.ecuManufacturer} onChange={(e)=>patch("ecuManufacturer",e.target.value)}/></label><label>Modelo ECU<input value={form.ecuModel} onChange={(e)=>patch("ecuModel",e.target.value)}/></label><label>Serial ECU<input value={form.ecuSerial} onChange={(e)=>patch("ecuSerial",e.target.value)}/></label><label>Protocolo ECU<input value={form.ecuProtocol} onChange={(e)=>patch("ecuProtocol",e.target.value)}/></label><label className="check-row"><input type="checkbox" checked={form.ecuJ1939} onChange={(e)=>patch("ecuJ1939",e.target.checked)}/> J1939 disponível</label>
      </div></Panel>
      <Panel title="Transporte / Rapid / Operação"><div className="admin-form-grid"><label>Meio<input value={form.transportKind} onChange={(e)=>patch("transportKind",e.target.value)}/></label><label>Gateway/rota<input value={form.gateway} onChange={(e)=>patch("gateway",e.target.value)}/></label><label>Host<input value={form.host} onChange={(e)=>patch("host",e.target.value)}/></label><label>Porta<input type="number" value={form.port} onChange={(e)=>patch("port",e.target.value)}/></label><label>Device/Unit ID<input value={form.deviceAddress} onChange={(e)=>patch("deviceAddress",e.target.value)}/></label><label>Notas<input value={form.notes} onChange={(e)=>patch("notes",e.target.value)}/></label><label>Rapid plan version<input value={form.rapidPlanVersion} onChange={(e)=>patch("rapidPlanVersion",e.target.value)}/></label><label>Rapid plan SHA-256<input value={form.rapidPlanHash} onChange={(e)=>patch("rapidPlanHash",e.target.value)}/></label><label>RC Monitor generatorId<input list="edit-monitor-generators" value={form.monitorGeneratorId} onChange={(e)=>patch("monitorGeneratorId",e.target.value)} placeholder="ex. gen-sim-001"/><datalist id="edit-monitor-generators">{candidates.map((generator)=><option key={generator.id} value={generator.id}>{generator.name} · {generator.siteId}</option>)}</datalist></label><label>Motivo da alteração<input value={form.changeReason} onChange={(e)=>patch("changeReason",e.target.value)}/></label></div><div className="wizard-note">Vincular um generatorId não promove o ativo. O backend ainda exige gates PASS, site coerente e telemetria required válida.</div><button className="primary-button" disabled={busy} onClick={()=>void save()}>{busy?"Salvando…":"Salvar revisão"}</button></Panel>
    </div>
  </>;
}

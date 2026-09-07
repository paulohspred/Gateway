import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { MapPinned, Pencil, Plus } from "lucide-react";
import { controlApi, type Site } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";

type SiteForm = { id?: string; code: string; name: string; client?: string; address?: string; latitude?: number; longitude?: number; technicalContact?: string; timeZone: string; active?: boolean };
export function SitesPage() {
  const auth = useAuth();
  const qc = useQueryClient();
  const query = useQuery({ queryKey: ["admin-sites"], queryFn: controlApi.sites });
  const [form, setForm] = useState<SiteForm>({ id: "", code: "", name: "", timeZone: "America/Sao_Paulo" });
  const [show, setShow] = useState(false);
  const [editing, setEditing] = useState<Site | null>(null);
  const [edit, setEdit] = useState<SiteForm | null>(null);
  const [message, setMessage] = useState("");
  const canWrite = auth.hasPermission("sites.write");
  const refresh = () => void qc.invalidateQueries({ queryKey: ["admin-sites"] });
  const create = useMutation({
    mutationFn: () => controlApi.createSite(auth.csrf, form),
    onSuccess: () => { setShow(false); setForm({ id: "", code: "", name: "", timeZone: "America/Sao_Paulo" }); setMessage("Site criado e auditado."); refresh(); },
    onError: (e) => setMessage(e instanceof Error ? e.message : "Falha ao criar site")
  });
  const beginEdit = (site: Site) => { setEditing(site); setEdit({ code: site.code, name: site.name, client: site.client ?? "", address: site.address ?? "", latitude: site.latitude, longitude: site.longitude, technicalContact: site.technicalContact ?? "", timeZone: site.timeZone, active: site.active }); setMessage(""); };
  const save = async () => { if (!editing || !edit) return; try { await controlApi.updateSite(auth.csrf, editing.id, edit); setEditing(null); setEdit(null); setMessage("Site atualizado e auditado."); refresh(); } catch (e) { setMessage(e instanceof Error ? e.message : "Falha ao atualizar site"); } };
  return <>
    <Topbar title="Sites" subtitle="Escopo administrativo e agrupamento de ativos" onRefresh={() => void query.refetch()} refreshing={query.isFetching}>
      {canWrite ? <button className="primary-button compact" onClick={() => setShow((v) => !v)}><Plus/>Novo site</button> : null}
    </Topbar>
    <div className="content-grid">
      {message ? <div className="info-banner">{message}</div> : null}
      {show ? <Panel title="Cadastrar site"><form className="admin-form-grid" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <label>ID de integração (opcional)<input value={form.id ?? ""} onChange={(e) => setForm({ ...form, id: e.target.value })} placeholder="ex. site-sim-001"/><small>Use o mesmo siteId do RC Monitor quando integrar uma frota existente.</small></label><label>Código<input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} required/></label>
        <label>Nome<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required/></label>
        <label>Cliente (opcional)<input value={form.client ?? ""} onChange={(e) => setForm({ ...form, client: e.target.value })}/></label>
        <label className="span-2">Endereço (opcional)<input value={form.address ?? ""} onChange={(e) => setForm({ ...form, address: e.target.value })}/></label>
        <label>Latitude<input type="number" step="any" min="-90" max="90" value={form.latitude ?? ""} onChange={(e) => setForm({ ...form, latitude: e.target.value === "" ? undefined : Number(e.target.value) })}/></label>
        <label>Longitude<input type="number" step="any" min="-180" max="180" value={form.longitude ?? ""} onChange={(e) => setForm({ ...form, longitude: e.target.value === "" ? undefined : Number(e.target.value) })}/></label>
        <label>Contato técnico<input value={form.technicalContact ?? ""} onChange={(e) => setForm({ ...form, technicalContact: e.target.value })}/></label>
        <label>Timezone<input value={form.timeZone} onChange={(e) => setForm({ ...form, timeZone: e.target.value })} required/></label>
        <button className="primary-button" disabled={create.isPending}>Criar site</button>
      </form></Panel> : null}
      {editing && edit ? <Panel title={`Editar site · ${editing.code}`}><div className="admin-form-grid">
        <label>Código<input value={edit.code} onChange={(e) => setEdit({ ...edit, code: e.target.value })}/></label>
        <label>Nome<input value={edit.name} onChange={(e) => setEdit({ ...edit, name: e.target.value })}/></label>
        <label>Cliente<input value={edit.client ?? ""} onChange={(e) => setEdit({ ...edit, client: e.target.value })}/></label>
        <label className="span-2">Endereço<input value={edit.address ?? ""} onChange={(e) => setEdit({ ...edit, address: e.target.value })}/></label>
        <label>Latitude<input type="number" step="any" min="-90" max="90" value={edit.latitude ?? ""} onChange={(e) => setEdit({ ...edit, latitude: e.target.value === "" ? undefined : Number(e.target.value) })}/></label>
        <label>Longitude<input type="number" step="any" min="-180" max="180" value={edit.longitude ?? ""} onChange={(e) => setEdit({ ...edit, longitude: e.target.value === "" ? undefined : Number(e.target.value) })}/></label>
        <label>Contato técnico<input value={edit.technicalContact ?? ""} onChange={(e) => setEdit({ ...edit, technicalContact: e.target.value })}/></label>
        <label>Timezone<input value={edit.timeZone} onChange={(e) => setEdit({ ...edit, timeZone: e.target.value })}/></label>
        <label className="check-row"><input type="checkbox" checked={edit.active ?? true} onChange={(e) => setEdit({ ...edit, active: e.target.checked })}/>Site ativo</label>
        <div className="row-actions"><button onClick={() => { setEditing(null); setEdit(null); }}>Cancelar</button><button className="primary-button" onClick={() => void save()}>Salvar alterações</button></div>
      </div></Panel> : null}
      <Panel title={`Sites (${query.data?.length ?? 0})`}><div className="site-grid">{query.data?.map((site) => <article className="site-card" key={site.id}>
        <MapPinned/><div><h3>{site.code}</h3><p>{site.name}</p>{site.client?<small>{site.client}</small>:null}{site.address?<small>{site.address}</small>:null}<small>{site.timeZone}</small>{site.technicalContact?<small>Contato: {site.technicalContact}</small>:null}</div><StatusBadge tone={site.active ? "healthy" : "offline"}>{site.active ? "ATIVO" : "INATIVO"}</StatusBadge>{canWrite ? <button onClick={() => beginEdit(site)}><Pencil/>Editar</button> : null}
      </article>)}</div></Panel>
    </div>
  </>;
}

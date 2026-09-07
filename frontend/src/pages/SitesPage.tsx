import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { MapPinned, Pencil, Plus } from "lucide-react";
import { controlApi, type Site } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";

type SiteForm = { id?: string; code: string; name: string; timeZone: string; active?: boolean };
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
  const beginEdit = (site: Site) => { setEditing(site); setEdit({ code: site.code, name: site.name, timeZone: site.timeZone, active: site.active }); setMessage(""); };
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
        <label>Timezone<input value={form.timeZone} onChange={(e) => setForm({ ...form, timeZone: e.target.value })} required/></label>
        <button className="primary-button" disabled={create.isPending}>Criar site</button>
      </form></Panel> : null}
      {editing && edit ? <Panel title={`Editar site · ${editing.code}`}><div className="admin-form-grid">
        <label>Código<input value={edit.code} onChange={(e) => setEdit({ ...edit, code: e.target.value })}/></label>
        <label>Nome<input value={edit.name} onChange={(e) => setEdit({ ...edit, name: e.target.value })}/></label>
        <label>Timezone<input value={edit.timeZone} onChange={(e) => setEdit({ ...edit, timeZone: e.target.value })}/></label>
        <label className="check-row"><input type="checkbox" checked={edit.active ?? true} onChange={(e) => setEdit({ ...edit, active: e.target.checked })}/>Site ativo</label>
        <div className="row-actions"><button onClick={() => { setEditing(null); setEdit(null); }}>Cancelar</button><button className="primary-button" onClick={() => void save()}>Salvar alterações</button></div>
      </div></Panel> : null}
      <Panel title={`Sites (${query.data?.length ?? 0})`}><div className="site-grid">{query.data?.map((site) => <article className="site-card" key={site.id}>
        <MapPinned/><div><h3>{site.code}</h3><p>{site.name}</p><small>{site.timeZone}</small></div><StatusBadge tone={site.active ? "healthy" : "offline"}>{site.active ? "ATIVO" : "INATIVO"}</StatusBadge>{canWrite ? <button onClick={() => beginEdit(site)}><Pencil/>Editar</button> : null}
      </article>)}</div></Panel>
    </div>
  </>;
}

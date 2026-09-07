import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { KeyRound, LogOut, RotateCcw, UserPen, UserPlus } from "lucide-react";
import { controlApi, type PublicUser, type Role } from "../api/control";
import { useAuth } from "../auth/AuthContext";
import { Panel } from "../components/Panel";
import { StatusBadge } from "../components/StatusBadge";
import { Topbar } from "../components/Topbar";
import { formatDateTime } from "../lib/time";

const labels: Record<Role, string> = {
  viewer: "Viewer",
  operator: "Operator",
  technician: "Technician",
  commissioning_engineer: "Commissioning Engineer",
  administrator: "Administrator",
  auditor: "Auditor"
};

type EditForm = { displayName: string; role: Role; siteScopes: string[]; active: boolean; mfaRequired: boolean };
const toEdit = (u: PublicUser): EditForm => ({ displayName: u.displayName, role: u.role, siteScopes: [...u.siteScopes], active: u.active, mfaRequired: u.mfaRequired });

export function UsersPage() {
  const auth = useAuth();
  const qc = useQueryClient();
  const users = useQuery({ queryKey: ["admin-users"], queryFn: controlApi.users });
  const roles = useQuery({ queryKey: ["admin-roles"], queryFn: controlApi.roles });
  const sites = useQuery({ queryKey: ["admin-sites"], queryFn: controlApi.sites });
  const [showCreate, setShowCreate] = useState(false);
  const [createdSecret, setCreatedSecret] = useState("");
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<PublicUser | null>(null);
  const [edit, setEdit] = useState<EditForm | null>(null);
  const [form, setForm] = useState({ username: "", displayName: "", role: "viewer" as Role, siteScopes: [] as string[], temporaryPassword: "", mfaRequired: false });
  const canWrite = auth.hasPermission("users.write");
  const rows = useMemo(() => users.data ?? [], [users.data]);
  const refresh = () => void qc.invalidateQueries({ queryKey: ["admin-users"] });
  const create = useMutation({
    mutationFn: () => controlApi.createUser(auth.csrf, form),
    onSuccess: (r) => {
      setCreatedSecret(r.temporaryPassword);
      setShowCreate(false);
      setForm({ username: "", displayName: "", role: "viewer", siteScopes: [], temporaryPassword: "", mfaRequired: false });
      refresh();
    },
    onError: (e) => setMessage(e instanceof Error ? e.message : "Falha ao criar usuário")
  });

  const toggleCreateSite = (id: string) => setForm((f) => ({ ...f, siteScopes: f.siteScopes.includes(id) ? f.siteScopes.filter((x) => x !== id) : [...f.siteScopes, id] }));
  const toggleEditSite = (id: string) => setEdit((f) => f ? ({ ...f, siteScopes: f.siteScopes.includes(id) ? f.siteScopes.filter((x) => x !== id) : [...f.siteScopes, id] }) : f);
  const beginEdit = (u: PublicUser) => { setEditing(u); setEdit(toEdit(u)); setMessage(""); };
  const saveEdit = async () => {
    if (!editing || !edit) return;
    try {
      await controlApi.updateUser(auth.csrf, editing.id, edit);
      setEditing(null); setEdit(null); setMessage("Usuário atualizado e auditado."); refresh();
      if (editing.id === auth.user?.id) await auth.refresh();
    } catch (e) { setMessage(e instanceof Error ? e.message : "Falha ao atualizar usuário"); }
  };
  const resetPassword = async (id: string) => { try { const r = await controlApi.resetPassword(auth.csrf, id); setCreatedSecret(r.temporaryPassword); refresh(); } catch (e) { setMessage(e instanceof Error ? e.message : "Falha"); } };
  const revoke = async (id: string) => { try { await controlApi.revokeSessions(auth.csrf, id); setMessage("Sessões revogadas."); refresh(); } catch (e) { setMessage(e instanceof Error ? e.message : "Falha"); } };
  const resetMfa = async (id: string) => { if (!confirm("Resetar o MFA deste usuário? As sessões serão revogadas e um novo enrollment será exigido quando aplicável.")) return; try { await controlApi.resetMFA(auth.csrf, id); setMessage("MFA resetado; novo enrollment será exigido conforme o perfil."); refresh(); } catch (e) { setMessage(e instanceof Error ? e.message : "Falha"); } };

  return <>
    <Topbar title="Usuários" subtitle="Identidades, perfis, MFA e escopo por site" onRefresh={() => void users.refetch()} refreshing={users.isFetching}>
      {canWrite ? <button className="primary-button compact" onClick={() => setShowCreate((v) => !v)}><UserPlus/>Novo usuário</button> : null}
    </Topbar>
    <div className="content-grid">
      {createdSecret ? <div className="secret-banner"><strong>Senha temporária — exibir uma única vez:</strong><code>{createdSecret}</code><button onClick={() => setCreatedSecret("")}>Fechar</button></div> : null}
      {message ? <div className="info-banner">{message}</div> : null}
      {showCreate ? <Panel title="Criar usuário"><form className="admin-form-grid" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <label>Usuário<input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required/></label>
        <label>Nome<input value={form.displayName} onChange={(e) => setForm({ ...form, displayName: e.target.value })} required/></label>
        <label>Perfil<select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value as Role })}>{roles.data?.map((r) => <option key={r.id} value={r.id}>{labels[r.id]}</option>)}</select></label>
        <label>Senha temporária (opcional)<input type="password" minLength={12} value={form.temporaryPassword} onChange={(e) => setForm({ ...form, temporaryPassword: e.target.value })} placeholder="Gerada automaticamente se vazia"/></label>
        <fieldset><legend>Sites</legend><label className="check-row"><input type="checkbox" checked={form.siteScopes.includes("*")} onChange={() => toggleCreateSite("*")}/>Todos os sites</label>{sites.data?.map((s) => <label className="check-row" key={s.id}><input type="checkbox" checked={form.siteScopes.includes(s.id)} disabled={form.siteScopes.includes("*")} onChange={() => toggleCreateSite(s.id)}/>{s.code} · {s.name}</label>)}</fieldset>
        <label className="check-row"><input type="checkbox" checked={form.mfaRequired} onChange={(e) => setForm({ ...form, mfaRequired: e.target.checked })}/>Exigir MFA</label>
        <button className="primary-button" disabled={create.isPending}>Criar usuário</button>
      </form></Panel> : null}
      {editing && edit ? <Panel title={`Editar · ${editing.username}`}><div className="admin-form-grid">
        <label>Nome<input value={edit.displayName} onChange={(e) => setEdit({ ...edit, displayName: e.target.value })}/></label>
        <label>Perfil<select value={edit.role} onChange={(e) => setEdit({ ...edit, role: e.target.value as Role })}>{roles.data?.map((r) => <option key={r.id} value={r.id}>{labels[r.id]}</option>)}</select></label>
        <fieldset><legend>Escopo de sites</legend><label className="check-row"><input type="checkbox" checked={edit.siteScopes.includes("*")} onChange={() => toggleEditSite("*")}/>Todos os sites</label>{sites.data?.map((s) => <label className="check-row" key={s.id}><input type="checkbox" checked={edit.siteScopes.includes(s.id)} disabled={edit.siteScopes.includes("*")} onChange={() => toggleEditSite(s.id)}/>{s.code} · {s.name}</label>)}</fieldset>
        <label className="check-row"><input type="checkbox" checked={edit.active} onChange={(e) => setEdit({ ...edit, active: e.target.checked })}/>Usuário ativo</label>
        <label className="check-row"><input type="checkbox" checked={edit.mfaRequired} disabled={edit.role === "administrator" || edit.role === "commissioning_engineer"} onChange={(e) => setEdit({ ...edit, mfaRequired: e.target.checked })}/>Exigir MFA</label>
        <div className="row-actions"><button onClick={() => { setEditing(null); setEdit(null); }}>Cancelar</button><button className="primary-button" onClick={() => void saveEdit()}>Salvar alterações</button></div>
      </div></Panel> : null}
      <Panel title={`Usuários (${rows.length})`}><div className="table-wrap"><table className="data-table"><thead><tr><th>Usuário</th><th>Perfil</th><th>Sites</th><th>MFA</th><th>Status</th><th>Último login</th><th>Ações</th></tr></thead><tbody>{rows.map((u) => <tr key={u.id}>
        <td><strong>{u.displayName}</strong><br/><small>{u.username}</small></td><td>{labels[u.role]}</td><td>{u.siteScopes.includes("*") ? "TODOS" : u.siteScopes.join(", ") || "—"}</td>
        <td><StatusBadge tone={u.mfaEnabled ? "healthy" : u.mfaRequired ? "warning" : "unknown"}>{u.mfaEnabled ? "ATIVO" : u.mfaRequired ? "PENDENTE" : "OPCIONAL"}</StatusBadge></td>
        <td><StatusBadge tone={u.active ? "healthy" : "offline"}>{u.active ? "ATIVO" : "INATIVO"}</StatusBadge></td><td>{formatDateTime(u.lastLoginAt)}</td>
        <td>{canWrite ? <div className="row-actions"><button onClick={() => beginEdit(u)} title="Editar"><UserPen/></button><button onClick={() => void resetPassword(u.id)} title="Resetar senha"><KeyRound/></button><button onClick={() => void resetMfa(u.id)} title="Resetar MFA"><RotateCcw/></button><button onClick={() => void revoke(u.id)} title="Revogar sessões"><LogOut/></button></div> : "—"}</td>
      </tr>)}</tbody></table></div></Panel>
    </div>
  </>;
}

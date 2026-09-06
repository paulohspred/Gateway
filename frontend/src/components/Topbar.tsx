import { RefreshCw, Search } from "lucide-react";
import type { ReactNode } from "react";

export function Topbar({ title, subtitle, onRefresh, refreshing, children }: { title: string; subtitle?: string; onRefresh?: () => void; refreshing?: boolean; children?: ReactNode }) {
  return <header className="topbar"><div><h1>{title}</h1>{subtitle ? <p>{subtitle}</p> : null}</div><div className="topbar-actions">{children}<div className="search-shell" aria-hidden="true"><Search/><span>Busca por gerador na lista</span></div>{onRefresh ? <button type="button" className="icon-button" onClick={onRefresh} disabled={refreshing} aria-label="Atualizar"><RefreshCw className={refreshing ? "spin" : ""}/></button> : null}</div></header>;
}

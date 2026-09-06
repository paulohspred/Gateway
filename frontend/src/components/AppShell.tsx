import { Activity, BellRing, CircuitBoard, Gauge, RadioTower, Zap } from "lucide-react";
import { NavLink, Outlet } from "react-router-dom";

const links = [
  { to: "/", label: "Visão Geral", icon: Gauge, end: true },
  { to: "/generators", label: "Geradores", icon: CircuitBoard },
  { to: "/alarms", label: "Alarmes", icon: BellRing },
  { to: "/events", label: "Eventos", icon: Activity },
  { to: "/communication", label: "Comunicação", icon: RadioTower }
];

export function AppShell() {
  return <div className="app-shell"><aside className="sidebar"><div className="brand"><Zap aria-hidden="true"/><div><strong>RC MONITOR</strong><span>Supervisão de grupos geradores</span></div></div><nav aria-label="Operação"><p className="nav-label">OPERAÇÃO</p>{links.map(({ to, label, icon: Icon, end }) => <NavLink key={to} to={to} end={end} className={({ isActive }) => isActive ? "nav-link active" : "nav-link"}><Icon aria-hidden="true"/><span>{label}</span></NavLink>)}</nav><div className="sidebar-footer"><span className="readonly-dot"/><div><strong>Somente leitura</strong><span>Comandos industriais bloqueados</span></div></div></aside><main className="main"><Outlet/></main></div>;
}

import { Navigate, Outlet, useLocation } from "react-router-dom";
import type { PublicUser } from "../api/control";
import { useAuth } from "./AuthContext";

export function securitySetupPending(user: PublicUser | null, mfaEnrollmentRequired: boolean) {
  return Boolean(user && (user.mustChangePassword || mfaEnrollmentRequired));
}

export function permissionAllowed(user: PublicUser | null, permission: string) {
  return Boolean(user?.permissions.includes(permission));
}

export function RequireAuth() {
  const auth = useAuth();
  const location = useLocation();
  if (auth.loading) return <div className="auth-loading">Carregando sessão…</div>;
  if (!auth.user) return <Navigate to="/login" replace state={{ from: location.pathname }}/>;
  if (securitySetupPending(auth.user, auth.mfaEnrollmentRequired) && location.pathname !== "/account/security") return <Navigate to="/account/security" replace/>;
  return <Outlet/>;
}

export function RequirePermission({ permission }: { permission: string }) {
  const auth = useAuth();
  if (!permissionAllowed(auth.user, permission)) return <Navigate to="/forbidden" replace/>;
  return <Outlet/>;
}

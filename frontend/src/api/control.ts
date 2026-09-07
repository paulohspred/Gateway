import { z } from "zod";
import { ApiError } from "./client";

export const roleSchema = z.enum(["viewer", "operator", "technician", "commissioning_engineer", "administrator", "auditor"]);
export const preferencesSchema = z.object({
  language: z.literal("pt-BR"),
  timeZone: z.string().min(1),
  unitSystem: z.literal("metric"),
  fleetView: z.enum(["vertical", "compact", "list"])
});
export const settingsSchema = z.object({
  defaultLanguage: z.literal("pt-BR"),
  defaultTimeZone: z.string().min(1),
  defaultFleetView: z.enum(["vertical", "compact", "list"])
});
export const publicUserSchema = z.object({
  id: z.string(), username: z.string(), displayName: z.string(), role: roleSchema,
  permissions: z.array(z.string()), siteScopes: z.array(z.string()), active: z.boolean(),
  mustChangePassword: z.boolean(), mfaRequired: z.boolean(), mfaEnabled: z.boolean(),
  createdAt: z.string(), updatedAt: z.string(), lastLoginAt: z.string().optional(), lockedUntil: z.string().optional(),
  preferences: preferencesSchema.optional()
});
export const authSessionSchema = z.object({ user: publicUserSchema, csrfToken: z.string(), expiresAt: z.string(), mfaEnrollmentRequired: z.boolean().optional().default(false) });
export const siteSchema = z.object({ id: z.string(), code: z.string(), name: z.string(), client: z.string().optional(), address: z.string().optional(), latitude: z.number().optional(), longitude: z.number().optional(), technicalContact: z.string().optional(), timeZone: z.string(), active: z.boolean(), createdAt: z.string(), updatedAt: z.string() });
export const gateSchema = z.object({ status: z.enum(["PENDING", "PASS", "FAIL", "BLOCKED"]), evidence: z.string().optional(), updatedAt: z.string(), updatedBy: z.string().optional() });
export const commissioningSchema = z.object({
  id: z.string(), tag: z.string(), name: z.string(), siteId: z.string(), lifecycle: z.string(), stage: z.string(),
  asset: z.record(z.string(), z.unknown()), controller: z.record(z.string(), z.unknown()), ecu: z.record(z.string(), z.unknown()).optional().default({}), transport: z.record(z.string(), z.unknown()),
  rapidPlanHash: z.string().optional(), rapidPlanVersion: z.string().optional(), monitorGeneratorId: z.string().optional(),
  parentCommissioningId: z.string().optional(), revision: z.number().int().optional().default(1), changeReason: z.string().optional(),
  gates: z.record(z.string(), gateSchema), createdBy: z.string(), createdAt: z.string(), updatedAt: z.string(), commissionedAt: z.string().optional()
});
export const profileCatalogSchema = z.object({ schema: z.number(), profiles: z.array(z.object({
  id: z.string(), manufacturer: z.string(), model: z.string(), displayName: z.string(), metrics: z.array(z.string()),
  alarms: z.array(z.object({ code: z.string(), severity: z.string(), message: z.string() }))
})) });
export const profileStateSchema = z.object({ profileId: z.string(), status: z.enum(["DRAFT", "LAB", "HIL_VALIDATED", "HOMOLOGATED", "DEPRECATED"]), version: z.string().optional(), evidence: z.string().optional(), updatedAt: z.string(), updatedBy: z.string() });
export const bindingSchema = z.object({
  profileId: z.string(), file: z.string(),
  metrics: z.array(z.object({ key: z.string(), rapidChannelNumber: z.number().int().positive() })),
  alarms: z.array(z.object({ code: z.string(), rapidChannelNumber: z.number().int().positive() })),
  events: z.array(z.object({ type: z.string(), rapidChannelNumber: z.number().int().positive() }))
});
export const preflightSchema = z.object({ commissioningId: z.string(), pass: z.boolean(), checks: z.array(z.object({ id: z.string(), status: z.enum(["PASS", "FAIL"]), message: z.string() })) });
export const telemetryValidationSchema = z.object({ pass: z.boolean(), communication: z.string(), requiredMetrics: z.record(z.string(), z.string()) });
export const auditSchema = z.object({
  id: z.string(), at: z.string(), actorUserId: z.string().optional(), actorUsername: z.string().optional(), action: z.string(),
  objectType: z.string(), objectId: z.string().optional(), result: z.string(), details: z.record(z.string(), z.string()).optional(),
  correlationId: z.string().optional(), source: z.string().optional()
});
export const commissioningEvidenceSchema = z.object({
  schema: z.literal(1), commissioning: commissioningSchema, profileState: profileStateSchema.optional(), audit: z.array(auditSchema), generatedAt: z.string()
});
export const systemInfoSchema = z.object({
  service: z.string(), version: z.string(), apiVersion: z.string(), cookieSecure: z.boolean(), sessionTtlSeconds: z.number(),
  profileCatalog: z.string(), bindingRoot: z.string().optional(), releaseVersion: z.string().optional(), commit: z.string().optional(), buildDate: z.string().optional(),
  nodeVersion: z.string().optional(), goVersion: z.string().optional(), rapidScadaVersion: z.string().optional(), gatewayVersion: z.string().optional(),
  monitorVersion: z.string().optional(), adminVersion: z.string().optional(), frontendVersion: z.string().optional(),
  gateway: z.object({ ready: z.boolean().optional(), operationalState: z.string().optional(), activeSessions: z.number().optional(), apiVersion: z.string().optional() }).nullable().optional(),
  monitor: z.unknown().nullable().optional()
});

export type PublicUser = z.infer<typeof publicUserSchema>;
export type Role = z.infer<typeof roleSchema>;
export type AuthSession = z.infer<typeof authSessionSchema>;
export type Site = z.infer<typeof siteSchema>;
export type Commissioning = z.infer<typeof commissioningSchema>;
export type Preferences = z.infer<typeof preferencesSchema>;
export type Settings = z.infer<typeof settingsSchema>;
export type ProfileState = z.infer<typeof profileStateSchema>;
export type BindingCatalogEntry = z.infer<typeof bindingSchema>;

async function request(path: string, init: RequestInit = {}, csrf?: string): Promise<unknown> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body) headers.set("Content-Type", "application/json");
  if (csrf) headers.set("X-CSRF-Token", csrf);
  const response = await fetch(path, { ...init, headers, credentials: "same-origin", cache: "no-store" });
  if (!response.ok) {
    if (response.status === 401 && typeof window !== "undefined") window.dispatchEvent(new Event("rc-auth-expired"));
    let code: string | undefined;
    let message = `HTTP ${response.status}`;
    try {
      const body = await response.json() as { error?: { code?: string; message?: string } };
      code = body.error?.code;
      message = body.error?.message || message;
    } catch { /* preserve HTTP-derived error */ }
    throw new ApiError(message, response.status, code);
  }
  if (response.status === 204) return null;
  return response.json();
}
const json = (value: unknown) => JSON.stringify(value);

export const controlApi = {
  login: async (username: string, password: string, totp?: string) => authSessionSchema.parse(await request("/api/v1/auth/login", { method: "POST", body: json({ username, password, ...(totp ? { totp } : {}) }) })),
  me: async () => authSessionSchema.parse(await request("/api/v1/auth/me")),
  logout: async (csrf: string) => request("/api/v1/auth/logout", { method: "POST" }, csrf),
  logoutAll: async (csrf: string) => request("/api/v1/auth/logout-all", { method: "POST" }, csrf),
  changePassword: async (csrf: string, currentPassword: string, newPassword: string) => request("/api/v1/auth/change-password", { method: "POST", body: json({ currentPassword, newPassword }) }, csrf),
  mfaSetup: async (csrf: string) => z.object({ secret: z.string(), otpauthUri: z.string() }).parse(await request("/api/v1/auth/mfa/setup", { method: "POST" }, csrf)),
  mfaEnable: async (csrf: string, code: string) => publicUserSchema.parse(await request("/api/v1/auth/mfa/enable", { method: "POST", body: json({ code }) }, csrf)),
  preferences: async () => preferencesSchema.parse(await request("/api/v1/auth/preferences")),
  updatePreferences: async (csrf: string, input: Preferences) => preferencesSchema.parse(await request("/api/v1/auth/preferences", { method: "PATCH", body: json(input) }, csrf)),

  roles: async () => z.array(z.object({ id: roleSchema, permissions: z.array(z.string()) })).parse(await request("/api/v1/admin/roles")),
  users: async () => z.array(publicUserSchema).parse(await request("/api/v1/admin/users")),
  createUser: async (csrf: string, input: unknown) => z.object({ user: publicUserSchema, temporaryPassword: z.string() }).parse(await request("/api/v1/admin/users", { method: "POST", body: json(input) }, csrf)),
  updateUser: async (csrf: string, id: string, input: unknown) => publicUserSchema.parse(await request(`/api/v1/admin/users/${encodeURIComponent(id)}`, { method: "PATCH", body: json(input) }, csrf)),
  resetPassword: async (csrf: string, id: string) => z.object({ temporaryPassword: z.string() }).parse(await request(`/api/v1/admin/users/${encodeURIComponent(id)}/reset-password`, { method: "POST" }, csrf)),
  revokeSessions: async (csrf: string, id: string) => request(`/api/v1/admin/users/${encodeURIComponent(id)}/revoke-sessions`, { method: "POST" }, csrf),
  resetMFA: async (csrf: string, id: string) => publicUserSchema.parse(await request(`/api/v1/admin/users/${encodeURIComponent(id)}/reset-mfa`, { method: "POST" }, csrf)),
  sites: async () => z.array(siteSchema).parse(await request("/api/v1/admin/sites")),
  createSite: async (csrf: string, input: unknown) => siteSchema.parse(await request("/api/v1/admin/sites", { method: "POST", body: json(input) }, csrf)),
  updateSite: async (csrf: string, id: string, input: unknown) => siteSchema.parse(await request(`/api/v1/admin/sites/${encodeURIComponent(id)}`, { method: "PATCH", body: json(input) }, csrf)),
  audit: async () => z.array(auditSchema).parse(await request("/api/v1/admin/audit?limit=500")),
  sessions: async () => z.array(z.object({ userId: z.string(), expiresAt: z.string(), current: z.boolean() })).parse(await request("/api/v1/admin/sessions")),
  diagnosticsSystem: async () => z.object({ gateway: z.object({ ready: z.boolean().optional(), operationalState: z.string().optional(), activeSessions: z.number().optional(), apiVersion: z.string().optional() }).nullable().optional(), monitor: z.unknown().nullable().optional() }).parse(await request("/api/v1/diagnostics/system")),
  system: async () => systemInfoSchema.parse(await request("/api/v1/admin/system")),
  settings: async () => settingsSchema.parse(await request("/api/v1/admin/settings")),
  updateSettings: async (csrf: string, input: Settings) => settingsSchema.parse(await request("/api/v1/admin/settings", { method: "PATCH", body: json(input) }, csrf)),

  monitorGenerators: async () => z.array(z.object({ id: z.string(), name: z.string(), siteId: z.string(), controller: z.object({ manufacturer: z.string(), model: z.string() }) }).passthrough()).parse(await request("/api/v1/engineering/monitor-generators")),
  profiles: async () => profileCatalogSchema.parse(await request("/api/v1/engineering/profiles")),
  profileStates: async () => z.array(profileStateSchema).parse(await request("/api/v1/engineering/profile-states")),
  setProfileState: async (csrf: string, profileId: string, input: { status: ProfileState["status"]; version: string; evidence: string }) => profileStateSchema.parse(await request(`/api/v1/engineering/profile-states/${encodeURIComponent(profileId)}`, { method: "PATCH", body: json(input) }, csrf)),
  bindings: async () => z.array(bindingSchema).parse(await request("/api/v1/engineering/bindings")),
  commissionings: async () => z.array(commissioningSchema).parse(await request("/api/v1/engineering/commissionings")),
  commissioning: async (id: string) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}`)),
  commissioningEvidence: async (id: string) => commissioningEvidenceSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/evidence`)),
  createCommissioning: async (csrf: string, input: unknown) => commissioningSchema.parse(await request("/api/v1/engineering/commissionings", { method: "POST", body: json(input) }, csrf)),
  updateCommissioning: async (csrf: string, id: string, input: unknown) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}`, { method: "PATCH", body: json(input) }, csrf)),
  preflightCommissioning: async (csrf: string, id: string) => preflightSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/preflight`, { method: "POST" }, csrf)),
  validateTelemetry: async (csrf: string, id: string) => telemetryValidationSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/validate-telemetry`, { method: "POST" }, csrf)),
  createChangeCommissioning: async (csrf: string, id: string, reason: string) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/change`, { method: "POST", body: json({ reason }) }, csrf)),
  setGate: async (csrf: string, id: string, stage: string, status: string, evidence: string) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/gates/${encodeURIComponent(stage)}`, { method: "POST", body: json({ status, evidence }) }, csrf)),
  promoteCommissioning: async (csrf: string, id: string) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/promote`, { method: "POST" }, csrf)),
  suspendCommissioning: async (csrf: string, id: string) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/suspend`, { method: "POST" }, csrf)),
  retireCommissioning: async (csrf: string, id: string) => commissioningSchema.parse(await request(`/api/v1/engineering/commissionings/${encodeURIComponent(id)}/retire`, { method: "POST" }, csrf))
};

import {
  alarmsSchema,
  eventsSchema,
  generatorSchema,
  generatorsSchema,
  systemHealthSchema,
  telemetrySchema,
  type Alarm,
  type Event,
  type Generator,
  type SystemHealth,
  type Telemetry
} from "./schemas";

const rawBase = (import.meta.env.VITE_RC_MONITOR_BASE_URL as string | undefined)?.trim() ?? "";
const API_BASE = rawBase.replace(/\/$/, "");

export class ApiError extends Error {
  constructor(message: string, readonly status: number, readonly code?: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function getJson(path: string): Promise<unknown> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: "GET",
    headers: { Accept: "application/json" },
    cache: "no-store"
  });

  if (!response.ok) {
    let code: string | undefined;
    let message = `HTTP ${response.status}`;
    try {
      const body = (await response.json()) as { error?: { code?: string; message?: string } };
      code = body.error?.code;
      message = body.error?.message || message;
    } catch {
      // Preserve the status-derived error when the body is not JSON.
    }
    throw new ApiError(message, response.status, code);
  }
  return response.json();
}

export const api = {
  listGenerators: async (): Promise<Generator[]> => generatorsSchema.parse(await getJson("/api/v1/generators")),
  getGenerator: async (id: string): Promise<Generator> => generatorSchema.parse(await getJson(`/api/v1/generators/${encodeURIComponent(id)}`)),
  getTelemetry: async (id: string): Promise<Telemetry> => telemetrySchema.parse(await getJson(`/api/v1/generators/${encodeURIComponent(id)}/telemetry`)),
  getAlarms: async (id: string): Promise<Alarm[]> => alarmsSchema.parse(await getJson(`/api/v1/generators/${encodeURIComponent(id)}/alarms`)),
  getEvents: async (id: string): Promise<Event[]> => eventsSchema.parse(await getJson(`/api/v1/generators/${encodeURIComponent(id)}/events`)),
  getSystemHealth: async (): Promise<SystemHealth> => systemHealthSchema.parse(await getJson("/api/v1/system/health"))
};

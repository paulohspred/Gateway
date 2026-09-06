import { z } from "zod";

export const qualitySchema = z.enum(["good", "stale", "offline", "bad", "unknown"]);
export const communicationSchema = z.enum(["online", "offline", "unknown"]);

export const controllerSchema = z.object({
  manufacturer: z.string(),
  model: z.string(),
  firmware: z.string().optional(),
  hardwareVersion: z.string().optional(),
  serialNumber: z.string().optional()
});

export const generatorSpecSchema = z.object({
  ratedPowerKw: z.number().nonnegative().optional(),
  nominalVoltage: z.number().positive().optional(),
  nominalFrequency: z.number().positive().optional(),
  nominalRpm: z.number().positive().optional(),
  phaseCount: z.union([z.literal(1), z.literal(3)]).optional()
});

export const generatorSchema = z.object({
  id: z.string().min(1),
  name: z.string().min(1),
  siteId: z.string().min(1),
  controller: controllerSchema,
  spec: generatorSpecSchema.optional()
});

export const metricSchema = z.object({
  value: z.union([z.number(), z.string(), z.boolean()]),
  unit: z.string().optional(),
  quality: qualitySchema,
  observedAt: z.string().datetime({ offset: true })
});

export const telemetrySchema = z.object({
  generatorId: z.string().min(1),
  capturedAt: z.string().datetime({ offset: true }),
  communication: communicationSchema,
  metrics: z.record(z.string(), metricSchema)
});

export const alarmSchema = z.object({
  id: z.string().min(1),
  generatorId: z.string().min(1),
  code: z.string(),
  severity: z.enum(["info", "warning", "critical"]),
  message: z.string(),
  active: z.boolean(),
  raisedAt: z.string().datetime({ offset: true }),
  clearedAt: z.string().datetime({ offset: true }).nullable().optional()
});

export const eventSchema = z.object({
  id: z.string().min(1),
  generatorId: z.string().min(1),
  type: z.string(),
  message: z.string(),
  occurredAt: z.string().datetime({ offset: true })
});

export const providerHealthSchema = z.object({
  status: z.enum(["healthy", "degraded", "unavailable"]),
  checkedAt: z.string().datetime({ offset: true }),
  message: z.string().optional()
});

export const systemHealthSchema = z.object({
  status: z.enum(["healthy", "degraded"]),
  apiVersion: z.string(),
  provider: providerHealthSchema
});

export const generatorsSchema = z.array(generatorSchema);
export const alarmsSchema = z.array(alarmSchema);
export const eventsSchema = z.array(eventSchema);

export type Generator = z.infer<typeof generatorSchema>;
export type Metric = z.infer<typeof metricSchema>;
export type Telemetry = z.infer<typeof telemetrySchema>;
export type Alarm = z.infer<typeof alarmSchema>;
export type Event = z.infer<typeof eventSchema>;
export type SystemHealth = z.infer<typeof systemHealthSchema>;

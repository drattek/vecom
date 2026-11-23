import { z } from 'zod'

// Fila de solo lectura de credenciales/ajustes de una conexión. Cuando
// `masked` es true el backend ya manda un valor oculto: el valor real nunca
// llega al navegador.
export const connectionConfigEntrySchema = z.object({
  keyName: z.string(),
  value: z.string(),
  masked: z.boolean(),
  isEncrypted: z.boolean(),
  updatedAt: z.string(),
})

export const connectionStatusSchema = z.object({
  authenticated: z.boolean(),
  lastAuth: z.string().nullish(),
  lastError: z.string().nullish(),
  expiresAt: z.string().nullish(),
  lastSync: z.string().nullish(),
})

export const connectionConfigSchema = z.object({
  connectionId: z.number(),
  credentials: z.array(connectionConfigEntrySchema),
  settings: z.array(connectionConfigEntrySchema),
  status: connectionStatusSchema.nullish(),
})

export const channelParameterSchema = z.object({
  id: z.number(),
  parameterName: z.string(),
  displayName: z.string(),
  parameterType: z.string(),
  required: z.boolean(),
  defaultValue: z.string().nullish(),
  isEncrypted: z.boolean(),
})

export const channelParametersResponseSchema = z.object({
  data: z.array(channelParameterSchema).nullish().transform((rows) => rows ?? []),
  total: z.number(),
})

export type ConnectionConfigEntry = z.infer<typeof connectionConfigEntrySchema>
export type ConnectionConfig = z.infer<typeof connectionConfigSchema>
export type ChannelParameter = z.infer<typeof channelParameterSchema>

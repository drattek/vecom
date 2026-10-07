import { z } from 'zod'

// Catálogos de compatibilidad (fitments) del core-orchestrator:
// /api/vehicle-fitments, /api/equipment-fitments y /api/equipment-types.
// `productCount` / `fitmentCount` son cuántos registros vivos dependen de la fila.

export const vehicleFitmentSchema = z.object({
  id: z.number(),
  brandId: z.number(),
  brandName: z.string(),
  model: z.string(),
  yearStart: z.number(),
  yearEnd: z.number().nullish(),
  productCount: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedVehicleFitmentsSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  fitments: z.array(vehicleFitmentSchema),
})

export const equipmentFitmentSchema = z.object({
  id: z.number(),
  brandId: z.number(),
  brandName: z.string(),
  equipmentTypeId: z.number(),
  equipmentTypeName: z.string(),
  model: z.string().nullish(),
  serie: z.string().nullish(),
  productCount: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedEquipmentFitmentsSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  fitments: z.array(equipmentFitmentSchema),
})

export const equipmentTypeSchema = z.object({
  id: z.number(),
  name: z.string(),
  fitmentCount: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedEquipmentTypesSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  equipmentTypes: z.array(equipmentTypeSchema),
})

export const findOrCreateVehicleFitmentResponseSchema = z.object({
  fitment: vehicleFitmentSchema,
  created: z.boolean(),
})

export const findOrCreateEquipmentFitmentResponseSchema = z.object({
  fitment: equipmentFitmentSchema,
  created: z.boolean(),
})

export type VehicleFitment = z.infer<typeof vehicleFitmentSchema>
export type EquipmentFitment = z.infer<typeof equipmentFitmentSchema>
export type EquipmentType = z.infer<typeof equipmentTypeSchema>

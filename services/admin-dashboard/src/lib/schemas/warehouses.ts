import { z } from 'zod'

export const warehouseSchema = z.object({
  id: z.number(),
  name: z.string(),
  branchId: z.number(),
  createdBy: z.number(),
  updatedBy: z.number().nullish(),
  createdAt: z.string(),
  updatedAt: z.string(),
  deletedAt: z.string().nullish(),
})

export const paginatedWarehousesResponseSchema = z.object({
  total: z.number(),
  offset: z.number(),
  pageSize: z.number(),
  warehouses: z.array(warehouseSchema),
})

export type Warehouse = z.infer<typeof warehouseSchema>
export type PaginatedWarehousesResponse = z.infer<typeof paginatedWarehousesResponseSchema>

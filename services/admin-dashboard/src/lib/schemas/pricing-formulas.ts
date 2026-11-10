import { z } from 'zod'

// Fórmula que convierte el precio base MXN de un producto en el precio que se
// envía a un marketplace. brandId/connectionId/priceListId en null actúan como
// comodín ("cualquiera"); la fila con los tres en null es el default universal.
export const pricingFormulaSchema = z.object({
  id: z.number(),
  brandId: z.number().nullable(),
  connectionId: z.number().nullable(),
  priceListId: z.number().nullable(),
  expression: z.string(),
  description: z.string().nullable(),
  createdBy: z.number(),
  updatedBy: z.number().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
})

export const paginatedPricingFormulasResponseSchema = z.object({
  data: z.array(pricingFormulaSchema).nullable().transform((value) => value ?? []),
  total: z.number(),
})

export type PricingFormula = z.infer<typeof pricingFormulaSchema>
export type PaginatedPricingFormulasResponse = z.infer<typeof paginatedPricingFormulasResponseSchema>

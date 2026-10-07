import { apiClient } from "@/lib/api"
import {
    equipmentFitmentSchema,
    equipmentTypeSchema,
    findOrCreateEquipmentFitmentResponseSchema,
    findOrCreateVehicleFitmentResponseSchema,
    paginatedEquipmentFitmentsSchema,
    paginatedEquipmentTypesSchema,
    paginatedVehicleFitmentsSchema,
    vehicleFitmentSchema,
    type EquipmentFitment,
    type VehicleFitment,
} from "@/lib/schemas/fitments"
import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query"

// Hooks y helpers de los catálogos de compatibilidad (vehículos / maquinaria) y
// del alta/baja de compatibilidades en un producto. Los usan la sección
// Compatibilidades del detalle de producto y Settings → Vehículos/Maquinaria.

export const MIN_FITMENT_YEAR = 1900
export const MAX_FITMENT_YEAR = 2100

// --- Valores de formulario ---------------------------------------------------

/**
 * Valor de un selector "existente o nuevo": `id` si se eligió una fila del
 * catálogo, o `id: null` + `name` si se escribió una nueva (el backend la crea
 * o reutiliza la que ya tenga ese nombre).
 */
export type CatalogValue = { id: number | null; name: string }

export const EMPTY_CATALOG_VALUE: CatalogValue = { id: null, name: "" }

export type VehicleFitmentDraft = {
    brand: CatalogValue
    model: string
    yearStart: string
    yearEnd: string
}

export type EquipmentFitmentDraft = {
    brand: CatalogValue
    type: CatalogValue
    model: string
    serie: string
}

export function emptyVehicleDraft(seed?: Partial<VehicleFitmentDraft>): VehicleFitmentDraft {
    return { brand: EMPTY_CATALOG_VALUE, model: "", yearStart: "", yearEnd: "", ...seed }
}

export function emptyEquipmentDraft(seed?: Partial<EquipmentFitmentDraft>): EquipmentFitmentDraft {
    return { brand: EMPTY_CATALOG_VALUE, type: EMPTY_CATALOG_VALUE, model: "", serie: "", ...seed }
}

export function vehicleDraftFromFitment(fitment: VehicleFitment): VehicleFitmentDraft {
    return {
        brand: { id: fitment.brandId, name: fitment.brandName },
        model: fitment.model,
        yearStart: String(fitment.yearStart),
        yearEnd: fitment.yearEnd ? String(fitment.yearEnd) : "",
    }
}

export function equipmentDraftFromFitment(fitment: EquipmentFitment): EquipmentFitmentDraft {
    return {
        brand: { id: fitment.brandId, name: fitment.brandName },
        type: { id: fitment.equipmentTypeId, name: fitment.equipmentTypeName },
        model: fitment.model ?? "",
        serie: fitment.serie ?? "",
    }
}

export type VehicleFitmentPayload = {
    brandId?: number
    brandName?: string
    model: string
    yearStart: number
    yearEnd: number | null
}

export type EquipmentFitmentPayload = {
    brandId?: number
    brandName?: string
    equipmentTypeId?: number
    equipmentTypeName?: string
    model: string | null
    serie: string | null
}

type Validation<T> = { ok: true; payload: T } | { ok: false; message: string }

function catalogRef(value: CatalogValue): { id?: number; name?: string } {
    if (value.id) {
        return { id: value.id }
    }
    const name = value.name.trim()
    return name ? { name } : {}
}

function parseYear(raw: string): number | null {
    const trimmed = raw.trim()
    if (!/^\d{4}$/.test(trimmed)) {
        return null
    }
    const year = Number(trimmed)
    return year >= MIN_FITMENT_YEAR && year <= MAX_FITMENT_YEAR ? year : null
}

export function validateVehicleDraft(draft: VehicleFitmentDraft): Validation<VehicleFitmentPayload> {
    const brand = catalogRef(draft.brand)
    if (!brand.id && !brand.name) {
        return { ok: false, message: "Elige o escribe una marca." }
    }

    const model = draft.model.trim()
    if (!model) {
        return { ok: false, message: "El modelo es obligatorio." }
    }

    const yearStart = parseYear(draft.yearStart)
    if (yearStart === null) {
        return { ok: false, message: `El año inicial debe ser de 4 dígitos (${MIN_FITMENT_YEAR}–${MAX_FITMENT_YEAR}).` }
    }

    let yearEnd: number | null = null
    if (draft.yearEnd.trim()) {
        yearEnd = parseYear(draft.yearEnd)
        if (yearEnd === null) {
            return { ok: false, message: `El año final debe ser de 4 dígitos (${MIN_FITMENT_YEAR}–${MAX_FITMENT_YEAR}).` }
        }
        if (yearEnd < yearStart) {
            return { ok: false, message: "El año final no puede ser menor al inicial." }
        }
    }

    return {
        ok: true,
        payload: { brandId: brand.id, brandName: brand.name, model, yearStart, yearEnd },
    }
}

export function validateEquipmentDraft(draft: EquipmentFitmentDraft): Validation<EquipmentFitmentPayload> {
    const brand = catalogRef(draft.brand)
    if (!brand.id && !brand.name) {
        return { ok: false, message: "Elige o escribe una marca." }
    }

    const type = catalogRef(draft.type)
    if (!type.id && !type.name) {
        return { ok: false, message: "Elige o escribe un tipo de maquinaria." }
    }

    const model = draft.model.trim() || null
    const serie = draft.serie.trim() || null
    if (!model && !serie) {
        return { ok: false, message: "Indica al menos el modelo o la serie." }
    }

    return {
        ok: true,
        payload: {
            brandId: brand.id,
            brandName: brand.name,
            equipmentTypeId: type.id,
            equipmentTypeName: type.name,
            model,
            serie,
        },
    }
}

// --- Formateadores -----------------------------------------------------------

/** Rango de años: "2015" o "2015–2018". */
export function formatYearRange(yearStart: number, yearEnd?: number | null): string {
    if (yearEnd && yearEnd !== yearStart) {
        return `${yearStart}–${yearEnd}`
    }
    return String(yearStart)
}

/** "Modelo X · Serie Y" con lo que haya (la maquinaria puede tener solo uno). */
export function formatEquipmentModel(model?: string | null, serie?: string | null): string {
    const parts = [model ? `Modelo ${model}` : null, serie ? `Serie ${serie}` : null].filter(Boolean)
    return parts.length > 0 ? parts.join(" · ") : "—"
}

export function formatProductCount(count: number): string {
    return count === 1 ? "1 producto" : `${count} productos`
}

// --- Consultas ---------------------------------------------------------------

export type FitmentListParams = {
    q: string
    brandId: number | null
    offset: number
    pageSize: number
}

export type EquipmentListParams = FitmentListParams & { equipmentTypeId: number | null }

function listParams(params: FitmentListParams & { equipmentTypeId?: number | null }) {
    return {
        offset: params.offset,
        pageSize: params.pageSize,
        q: params.q.trim() || undefined,
        brandId: params.brandId ?? undefined,
        equipmentTypeId: params.equipmentTypeId ?? undefined,
    }
}

export function useVehicleFitments(params: FitmentListParams, enabled = true) {
    return useQuery({
        queryKey: ["fitments", "vehicle", params],
        enabled,
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/vehicle-fitments", { params: listParams(params) })
            return paginatedVehicleFitmentsSchema.parse(response.data)
        },
    })
}

export function useEquipmentFitments(params: EquipmentListParams, enabled = true) {
    return useQuery({
        queryKey: ["fitments", "equipment", params],
        enabled,
        placeholderData: keepPreviousData,
        queryFn: async () => {
            const response = await apiClient.get("/api/equipment-fitments", { params: listParams(params) })
            return paginatedEquipmentFitmentsSchema.parse(response.data)
        },
    })
}

/** Todos los tipos de maquinaria (el catálogo es corto) para selectores y la página de tipos. */
export function useEquipmentTypes(enabled = true) {
    return useQuery({
        queryKey: ["equipment-types", "all"],
        enabled,
        staleTime: 60_000,
        queryFn: async () => {
            const response = await apiClient.get("/api/equipment-types", { params: { pageSize: 100 } })
            return paginatedEquipmentTypesSchema.parse(response.data).equipmentTypes
        },
    })
}

// --- Mutaciones del catálogo -------------------------------------------------

/**
 * Los fitments también pueden crear marcas y tipos al vuelo, y sus conteos
 * (productos / fitments) cambian con cada alta o baja: se invalida todo el
 * catálogo junto.
 */
function invalidateCatalog(queryClient: QueryClient) {
    queryClient.invalidateQueries({ queryKey: ["fitments"] })
    queryClient.invalidateQueries({ queryKey: ["equipment-types"] })
    queryClient.invalidateQueries({ queryKey: ["brand-options"] })
    queryClient.invalidateQueries({ queryKey: ["brands"] })
}

/** Alta que reutiliza el fitment si ya existe (POST /api/vehicle-fitments/find-or-create). */
export function useFindOrCreateVehicleFitment() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (payload: VehicleFitmentPayload) => {
            const response = await apiClient.post("/api/vehicle-fitments/find-or-create", payload)
            return findOrCreateVehicleFitmentResponseSchema.parse(response.data)
        },
        onSuccess: () => invalidateCatalog(queryClient),
    })
}

/** Alta que reutiliza el fitment si ya existe (POST /api/equipment-fitments/find-or-create). */
export function useFindOrCreateEquipmentFitment() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (payload: EquipmentFitmentPayload) => {
            const response = await apiClient.post("/api/equipment-fitments/find-or-create", payload)
            return findOrCreateEquipmentFitmentResponseSchema.parse(response.data)
        },
        onSuccess: () => invalidateCatalog(queryClient),
    })
}

/** Edición de un fitment de vehículo (PUT /api/vehicle-fitments/{id}); la marca va por id. */
export function useUpdateVehicleFitment() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { id: number; brandId: number; model: string; yearStart: number; yearEnd: number | null }) => {
            const { id, ...body } = input
            const response = await apiClient.put(`/api/vehicle-fitments/${id}`, body)
            return vehicleFitmentSchema.parse(response.data)
        },
        onSuccess: () => invalidateCatalog(queryClient),
    })
}

/** Edición de un fitment de maquinaria (PUT /api/equipment-fitments/{id}); marca y tipo van por id. */
export function useUpdateEquipmentFitment() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: {
            id: number
            brandId: number
            equipmentTypeId: number
            model: string | null
            serie: string | null
        }) => {
            const { id, ...body } = input
            const response = await apiClient.put(`/api/equipment-fitments/${id}`, body)
            return equipmentFitmentSchema.parse(response.data)
        },
        onSuccess: () => invalidateCatalog(queryClient),
    })
}

/**
 * Baja de un fitment. Con `force` también desvincula los productos que lo usan
 * (sin `force` el backend responde 409 si hay alguno).
 */
export function useDeleteVehicleFitment() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { id: number; force: boolean }) => {
            await apiClient.delete(`/api/vehicle-fitments/${input.id}`, { params: input.force ? { force: true } : undefined })
        },
        onSuccess: () => {
            invalidateCatalog(queryClient)
            queryClient.invalidateQueries({ queryKey: ["product-details"] })
        },
    })
}

export function useDeleteEquipmentFitment() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { id: number; force: boolean }) => {
            await apiClient.delete(`/api/equipment-fitments/${input.id}`, { params: input.force ? { force: true } : undefined })
        },
        onSuccess: () => {
            invalidateCatalog(queryClient)
            queryClient.invalidateQueries({ queryKey: ["product-details"] })
        },
    })
}

/** Alta o edición de un tipo de maquinaria (POST / PUT /api/equipment-types). */
export function useSaveEquipmentType() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: { id?: number; name: string }) => {
            const response = input.id
                ? await apiClient.put(`/api/equipment-types/${input.id}`, { name: input.name })
                : await apiClient.post("/api/equipment-types", { name: input.name })
            return equipmentTypeSchema.parse(response.data)
        },
        onSuccess: () => invalidateCatalog(queryClient),
    })
}

/** Baja de un tipo; el backend responde 409 mientras tenga fitments. */
export function useDeleteEquipmentType() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (id: number) => {
            await apiClient.delete(`/api/equipment-types/${id}`)
        },
        onSuccess: () => invalidateCatalog(queryClient),
    })
}

// --- Compatibilidades de un producto -----------------------------------------

function invalidateProductCompatibilities(queryClient: QueryClient, productId: string | undefined) {
    queryClient.invalidateQueries({ queryKey: ["product-details", productId, "compatibilities"] })
    // El conteo de productos por fitment cambia con cada alta/baja de enlace.
    queryClient.invalidateQueries({ queryKey: ["fitments"] })
}

export type VehicleLinkInput = { vehicleFitmentId: number; motor: string; position: string; side: string }

/** Vincula el producto a un fitment de vehículo (POST /api/product-vehicle-compatibilities). */
export function useLinkVehicleCompatibility(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (input: VehicleLinkInput) => {
            await apiClient.post("/api/product-vehicle-compatibilities", {
                productId: Number(productId),
                ...input,
            })
        },
        onSuccess: () => invalidateProductCompatibilities(queryClient, productId),
    })
}

/** Vincula el producto a un fitment de maquinaria (POST /api/product-equipment-compatibilities). */
export function useLinkEquipmentCompatibility(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (equipmentFitmentId: number) => {
            await apiClient.post("/api/product-equipment-compatibilities", {
                productId: Number(productId),
                equipmentFitmentId,
            })
        },
        onSuccess: () => invalidateProductCompatibilities(queryClient, productId),
    })
}

/** Quita el vínculo producto↔vehículo (DELETE /api/product-vehicle-compatibilities/{id}); no borra el fitment. */
export function useUnlinkVehicleCompatibility(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (compatibilityId: number) => {
            await apiClient.delete(`/api/product-vehicle-compatibilities/${compatibilityId}`)
        },
        onSuccess: () => invalidateProductCompatibilities(queryClient, productId),
    })
}

/** Quita el vínculo producto↔maquinaria (DELETE /api/product-equipment-compatibilities/{id}); no borra el fitment. */
export function useUnlinkEquipmentCompatibility(productId: string | undefined) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (compatibilityId: number) => {
            await apiClient.delete(`/api/product-equipment-compatibilities/${compatibilityId}`)
        },
        onSuccess: () => invalidateProductCompatibilities(queryClient, productId),
    })
}

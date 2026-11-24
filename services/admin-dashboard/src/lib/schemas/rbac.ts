import { z } from 'zod'

export const permissionSchema = z.object({
  id: z.number(),
  resource: z.string(),
  action: z.string(),
  description: z.string(),
})

export const roleSchema = z.object({
  id: z.number(),
  code: z.string(),
  name: z.string(),
  description: z.string(),
  permissionCount: z.number(),
  userCount: z.number(),
  permissionIds: z.array(z.number()),
})

export const rolesResponseSchema = z.object({ roles: z.array(roleSchema) })
export const permissionsResponseSchema = z.object({ permissions: z.array(permissionSchema) })

export const permissionOverrideSchema = z.object({
  permissionId: z.number(),
  effect: z.enum(['grant', 'deny']),
})

export const userDetailSchema = z.object({
  id: z.number(),
  username: z.string(),
  role: z.string(),
  roleId: z.number().nullable(),
  roleName: z.string(),
  isActive: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
  overrides: z.array(permissionOverrideSchema),
})

export type Permission = z.infer<typeof permissionSchema>
export type Role = z.infer<typeof roleSchema>
export type PermissionOverride = z.infer<typeof permissionOverrideSchema>
export type UserDetail = z.infer<typeof userDetailSchema>

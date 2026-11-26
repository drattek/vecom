import type { ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getServerErrorMessage } from "@/lib/api"

const SKELETON_ROWS = 4

/** Error de un bloque, con reintento propio: no afecta al resto de la pantalla. */
export function BlockError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
    return (
        <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-destructive">
            <span>{getServerErrorMessage(error, "No se pudo cargar esta sección.")}</span>
            <Button variant="outline" size="sm" onClick={onRetry}>
                Reintentar
            </Button>
        </div>
    )
}

type DashboardTableProps = {
    columns: string[]
    isLoading: boolean
    /** Refrescando con datos anteriores a la vista (cambio de página). */
    isStale?: boolean
    isEmpty: boolean
    emptyMessage: string
    children: ReactNode
}

/** Tabla de un bloque del Dashboard, con skeleton mientras carga. */
export function DashboardTable({ columns, isLoading, isStale, isEmpty, emptyMessage, children }: DashboardTableProps) {
    return (
        <Table
            containerClassName={isStale ? "opacity-60 transition-opacity" : "transition-opacity"}
            aria-busy={isLoading || isStale}
        >
            <TableHeader>
                <TableRow>
                    {columns.map((column) => (
                        <TableHead key={column}>{column}</TableHead>
                    ))}
                </TableRow>
            </TableHeader>
            <TableBody>
                {isLoading
                    ? Array.from({ length: SKELETON_ROWS }, (_, row) => (
                          <TableRow key={row}>
                              {columns.map((column) => (
                                  <TableCell key={column}>
                                      <Skeleton className="h-4 w-full min-w-16" />
                                  </TableCell>
                              ))}
                          </TableRow>
                      ))
                    : null}
                {!isLoading && isEmpty ? (
                    <TableRow>
                        <TableCell colSpan={columns.length} className="text-center text-muted-foreground">
                            {emptyMessage}
                        </TableCell>
                    </TableRow>
                ) : null}
                {isLoading ? null : children}
            </TableBody>
        </Table>
    )
}

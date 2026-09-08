import { Button } from "@/components/ui/button"
import { getServerErrorMessage } from "@/lib/api"
import { fetchCategoryImportTree } from "@/lib/categories"
import type { CategoryImportNode } from "@/lib/schemas/categories"
import { cn } from "@/lib/utils"
import { ChevronRightIcon } from "lucide-react"
import { useEffect, useState } from "react"

const ROOT_KEY = ""

/**
 * Navegador lazy del árbol de categorías **externas** de una conexión. Reusado
 * por el modal de importar y por el de vincular/reemplazar un mapeo. Se monta
 * con `key={connectionId}` para arrancar con estado fresco por conexión.
 * `onPick` se dispara al pulsar el botón de una hoja; el padre decide qué hace
 * (importar / vincular) y pasa `pickingExternalId` mientras trabaja.
 */
export function ExternalCategoryTree({
    connectionId,
    onPick,
    pickLabel = "Importar",
    pickingExternalId = null,
    busy = false,
}: {
    connectionId: number
    onPick: (node: CategoryImportNode) => void
    pickLabel?: string
    pickingExternalId?: string | null
    busy?: boolean
}) {
    const [childrenByKey, setChildrenByKey] = useState<Map<string, CategoryImportNode[]>>(new Map())
    const [loadingKeys, setLoadingKeys] = useState<Set<string>>(() => new Set([ROOT_KEY]))
    const [expandedKeys, setExpandedKeys] = useState<Set<string>>(new Set())
    const [inferredLeaf, setInferredLeaf] = useState<Set<string>>(new Set())
    const [browseError, setBrowseError] = useState<string | null>(null)

    useEffect(() => {
        let cancelled = false
        fetchCategoryImportTree(connectionId)
            .then((nodes) => {
                if (!cancelled) setChildrenByKey(new Map([[ROOT_KEY, nodes]]))
            })
            .catch((error) => {
                if (!cancelled) {
                    setBrowseError(getServerErrorMessage(error, "No se pudo cargar el árbol del canal."))
                }
            })
            .finally(() => {
                if (!cancelled) {
                    setLoadingKeys((prev) => {
                        const next = new Set(prev)
                        next.delete(ROOT_KEY)
                        return next
                    })
                }
            })
        return () => {
            cancelled = true
        }
    }, [connectionId])

    async function loadChildren(key: string) {
        if (childrenByKey.has(key) || loadingKeys.has(key)) {
            return
        }
        setLoadingKeys((prev) => new Set(prev).add(key))
        setBrowseError(null)
        try {
            const nodes = await fetchCategoryImportTree(connectionId, key)
            setChildrenByKey((prev) => new Map(prev).set(key, nodes))
            if (nodes.length === 0) {
                setInferredLeaf((prev) => new Set(prev).add(key))
            }
        } catch (error) {
            setBrowseError(getServerErrorMessage(error, "No se pudo cargar esta rama."))
        } finally {
            setLoadingKeys((prev) => {
                const next = new Set(prev)
                next.delete(key)
                return next
            })
        }
    }

    function toggle(key: string) {
        setExpandedKeys((prev) => {
            const next = new Set(prev)
            if (next.has(key)) {
                next.delete(key)
            } else {
                next.add(key)
                void loadChildren(key)
            }
            return next
        })
    }

    const roots = childrenByKey.get(ROOT_KEY) ?? []

    return (
        <div className="flex min-h-0 flex-1 flex-col gap-2">
            {browseError ? <p className="text-xs text-destructive">{browseError}</p> : null}
            <div className="min-h-48 flex-1 overflow-auto rounded-lg border border-border">
                {loadingKeys.has(ROOT_KEY) ? (
                    <p className="py-6 text-center text-sm text-muted-foreground">Cargando…</p>
                ) : roots.length === 0 ? (
                    <p className="py-6 text-center text-sm text-muted-foreground">
                        {browseError ? "—" : "Sin categorías."}
                    </p>
                ) : (
                    <ul className="py-1 text-sm">
                        {roots.map((node) => (
                            <Node
                                key={node.externalId}
                                node={node}
                                depth={0}
                                childrenByKey={childrenByKey}
                                loadingKeys={loadingKeys}
                                expandedKeys={expandedKeys}
                                inferredLeaf={inferredLeaf}
                                pickLabel={pickLabel}
                                pickingExternalId={pickingExternalId}
                                busy={busy}
                                onToggle={toggle}
                                onPick={onPick}
                            />
                        ))}
                    </ul>
                )}
            </div>
        </div>
    )
}

function Node({
    node,
    depth,
    childrenByKey,
    loadingKeys,
    expandedKeys,
    inferredLeaf,
    pickLabel,
    pickingExternalId,
    busy,
    onToggle,
    onPick,
}: {
    node: CategoryImportNode
    depth: number
    childrenByKey: Map<string, CategoryImportNode[]>
    loadingKeys: Set<string>
    expandedKeys: Set<string>
    inferredLeaf: Set<string>
    pickLabel: string
    pickingExternalId: string | null
    busy: boolean
    onToggle: (key: string) => void
    onPick: (node: CategoryImportNode) => void
}) {
    const key = node.externalId
    const isLeaf = node.leaf === true || inferredLeaf.has(key)
    const isExpanded = expandedKeys.has(key)
    const isLoading = loadingKeys.has(key)
    const children = childrenByKey.get(key) ?? []
    const paddingLeft = 8 + depth * 16

    return (
        <li>
            <div className="flex items-center gap-1.5 pr-2 hover:bg-muted/60" style={{ paddingLeft }}>
                {isLeaf ? (
                    <span className="size-5 shrink-0" />
                ) : (
                    <button
                        type="button"
                        onClick={() => onToggle(key)}
                        aria-expanded={isExpanded}
                        aria-label={isExpanded ? `Colapsar ${node.name}` : `Expandir ${node.name}`}
                        className="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground hover:text-foreground"
                    >
                        <ChevronRightIcon className={cn("size-4 transition-transform", isExpanded && "rotate-90")} />
                    </button>
                )}

                <span className="min-w-0 flex-1 truncate py-1.5 text-foreground">
                    {node.name}
                    <span className="ml-1.5 text-xs text-muted-foreground">{node.externalId}</span>
                </span>

                {isLeaf ? (
                    <Button
                        variant="outline"
                        size="sm"
                        className="h-6 px-2 text-xs"
                        disabled={busy}
                        onClick={() => onPick(node)}
                    >
                        {pickingExternalId === key ? "…" : pickLabel}
                    </Button>
                ) : null}
            </div>

            {isExpanded ? (
                <ul>
                    {isLoading ? (
                        <li className="py-1 text-xs text-muted-foreground" style={{ paddingLeft: paddingLeft + 20 }}>
                            Cargando…
                        </li>
                    ) : children.length === 0 ? (
                        <li className="py-1 text-xs text-muted-foreground" style={{ paddingLeft: paddingLeft + 20 }}>
                            Sin subcategorías.
                        </li>
                    ) : (
                        children.map((child) => (
                            <Node
                                key={child.externalId}
                                node={child}
                                depth={depth + 1}
                                childrenByKey={childrenByKey}
                                loadingKeys={loadingKeys}
                                expandedKeys={expandedKeys}
                                inferredLeaf={inferredLeaf}
                                pickLabel={pickLabel}
                                pickingExternalId={pickingExternalId}
                                busy={busy}
                                onToggle={onToggle}
                                onPick={onPick}
                            />
                        ))
                    )}
                </ul>
            ) : null}
        </li>
    )
}

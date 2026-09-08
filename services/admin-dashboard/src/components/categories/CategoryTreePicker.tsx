import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogClose,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { useCategoryTree, type CategoryNode } from "@/lib/categories"
import { cn } from "@/lib/utils"
import { CheckIcon, ChevronRightIcon, FolderTreeIcon } from "lucide-react"
import { useMemo, useState } from "react"

/**
 * Selector de categoría en árbol donde **cualquier** nodo es elegible (rama u
 * hoja), más una opción "sin padre". Generaliza CategoryPickerDialog (que solo
 * deja elegir hojas y usa 0 = "sin categoría"). `value` es el id o null;
 * `excludeId` oculta esa categoría y toda su descendencia (para no elegirse a sí
 * misma como padre).
 */
export function CategoryTreePicker({
    value,
    onChange,
    excludeId,
    disabled,
    rootLabel = "Sin categoría padre",
}: {
    value: number | null
    onChange: (categoryId: number | null) => void
    excludeId?: number
    disabled?: boolean
    rootLabel?: string
}) {
    const [open, setOpen] = useState(false)
    const { data, isLoading, isError } = useCategoryTree()

    const excluded = useMemo(() => {
        const ids = new Set<number>()
        if (excludeId != null && data) {
            const walk = (node?: CategoryNode) => {
                if (!node) return
                ids.add(node.id)
                for (const child of node.children) {
                    walk(child)
                }
            }
            walk(data.byId.get(excludeId))
        }
        return ids
    }, [data, excludeId])

    const triggerLabel =
        value != null ? (data?.byId.get(value)?.path ?? `Categoría #${value}`) : rootLabel

    const ancestorIds = useMemo(() => {
        const ids = new Set<number>()
        if (data && value != null) {
            let node = data.byId.get(value)
            while (node?.parentId != null) {
                ids.add(node.parentId)
                node = data.byId.get(node.parentId)
            }
        }
        return ids
    }, [data, value])

    const [expanded, setExpanded] = useState<Set<number>>(new Set())

    function handleOpenChange(next: boolean) {
        if (next) {
            setExpanded(new Set(ancestorIds))
        }
        setOpen(next)
    }

    function toggle(id: number) {
        setExpanded((prev) => {
            const next = new Set(prev)
            if (next.has(id)) {
                next.delete(id)
            } else {
                next.add(id)
            }
            return next
        })
    }

    function select(categoryId: number | null) {
        onChange(categoryId)
        setOpen(false)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button variant="outline" className="w-full justify-between font-normal" disabled={disabled}>
                        <span className="truncate">{triggerLabel}</span>
                        <FolderTreeIcon className="text-muted-foreground" />
                    </Button>
                }
            />
            <DialogContent className="max-h-[80dvh] w-[40vw] min-w-[20rem] max-w-none">
                <DialogHeader>
                    <DialogTitle>Seleccionar categoría padre</DialogTitle>
                </DialogHeader>

                <div className="-mx-2 min-h-0 flex-1 overflow-y-auto px-2">
                    {isLoading ? (
                        <p className="py-6 text-center text-sm text-muted-foreground">Cargando categorías…</p>
                    ) : null}
                    {isError ? (
                        <p className="py-6 text-center text-sm text-destructive">
                            No se pudieron cargar las categorías.
                        </p>
                    ) : null}
                    {data ? (
                        <ul className="text-sm">
                            <li>
                                <button
                                    type="button"
                                    onClick={() => select(null)}
                                    className={cn(
                                        "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left hover:bg-muted",
                                        value == null ? "font-medium text-foreground" : "text-muted-foreground",
                                    )}
                                >
                                    <span className="flex size-4 shrink-0 items-center justify-center">
                                        {value == null ? <CheckIcon className="size-3.5" /> : null}
                                    </span>
                                    {rootLabel}
                                </button>
                            </li>
                            {data.tree
                                .filter((node) => !excluded.has(node.id))
                                .map((node) => (
                                    <TreeNode
                                        key={node.id}
                                        node={node}
                                        depth={0}
                                        value={value}
                                        excluded={excluded}
                                        expanded={expanded}
                                        onToggle={toggle}
                                        onSelect={select}
                                    />
                                ))}
                        </ul>
                    ) : null}
                </div>

                <DialogFooter>
                    <DialogClose render={<Button variant="ghost" size="sm">Cerrar</Button>} />
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function TreeNode({
    node,
    depth,
    value,
    excluded,
    expanded,
    onToggle,
    onSelect,
}: {
    node: CategoryNode
    depth: number
    value: number | null
    excluded: Set<number>
    expanded: Set<number>
    onToggle: (id: number) => void
    onSelect: (id: number) => void
}) {
    const childNodes = node.children.filter((child) => !excluded.has(child.id))
    const hasChildren = childNodes.length > 0
    const isExpanded = hasChildren && expanded.has(node.id)
    const isSelected = node.id === value
    const paddingLeft = 8 + depth * 16

    return (
        <li>
            <div className="flex items-center gap-1 pr-2 hover:bg-muted" style={{ paddingLeft }}>
                {hasChildren ? (
                    <button
                        type="button"
                        onClick={() => onToggle(node.id)}
                        aria-expanded={isExpanded}
                        aria-label={isExpanded ? `Colapsar ${node.name}` : `Expandir ${node.name}`}
                        className="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground hover:text-foreground"
                    >
                        <ChevronRightIcon className={cn("size-4 transition-transform", isExpanded && "rotate-90")} />
                    </button>
                ) : (
                    <span className="size-5 shrink-0" />
                )}

                <button
                    type="button"
                    onClick={() => onSelect(node.id)}
                    className={cn(
                        "flex min-w-0 flex-1 items-center gap-2 rounded-md py-1.5 pr-2 text-left",
                        isSelected ? "font-medium text-foreground" : "text-foreground",
                    )}
                >
                    <span className="flex size-4 shrink-0 items-center justify-center">
                        {isSelected ? <CheckIcon className="size-3.5" /> : null}
                    </span>
                    <span className="truncate">{node.name}</span>
                </button>
            </div>

            {isExpanded ? (
                <ul>
                    {childNodes.map((child) => (
                        <TreeNode
                            key={child.id}
                            node={child}
                            depth={depth + 1}
                            value={value}
                            excluded={excluded}
                            expanded={expanded}
                            onToggle={onToggle}
                            onSelect={onSelect}
                        />
                    ))}
                </ul>
            ) : null}
        </li>
    )
}

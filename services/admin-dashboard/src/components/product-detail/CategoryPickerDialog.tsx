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
import { useCategoryTree, type CategoryNode } from "@/lib/product-detail"
import { cn } from "@/lib/utils"
import { CheckIcon, ChevronRightIcon, FolderIcon } from "lucide-react"
import { useMemo, useState } from "react"

// value 0 = "sin categoría". Solo las hojas del árbol son asignables: los nodos
// con hijos solo se expanden/colapsan.
export function CategoryPickerDialog({
    value,
    onChange,
    disabled,
}: {
    value: number
    onChange: (categoryId: number) => void
    disabled?: boolean
}) {
    const [open, setOpen] = useState(false)
    const { data, isLoading, isError } = useCategoryTree()

    const selectedPath = value > 0 ? data?.byId.get(value)?.path : null
    const triggerLabel = value > 0 ? (selectedPath ?? `Categoría #${value}`) : "Sin categoría"

    // Ids de los ancestros de la categoría seleccionada, para abrir el árbol ya
    // desplegado hasta ella.
    const ancestorIds = useMemo(() => {
        const ids = new Set<number>()
        if (data && value > 0) {
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

    function select(categoryId: number) {
        onChange(categoryId)
        setOpen(false)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={
                    <Button
                        variant="outline"
                        className="w-full justify-between font-normal"
                        disabled={disabled}
                    >
                        <span className="truncate">{triggerLabel}</span>
                        <FolderIcon className="text-muted-foreground" />
                    </Button>
                }
            />
            <DialogContent className="max-h-[80dvh] w-[40vw] min-w-[20rem] max-w-none">
                <DialogHeader>
                    <DialogTitle>Seleccionar categoría</DialogTitle>
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
                                    onClick={() => select(0)}
                                    className={cn(
                                        "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left hover:bg-muted",
                                        value === 0 ? "font-medium text-foreground" : "text-muted-foreground",
                                    )}
                                >
                                    <span className="flex size-4 shrink-0 items-center justify-center">
                                        {value === 0 ? <CheckIcon className="size-3.5" /> : null}
                                    </span>
                                    Sin categoría
                                </button>
                            </li>
                            {data.tree.map((node) => (
                                <CategoryTreeNode
                                    key={node.id}
                                    node={node}
                                    depth={0}
                                    value={value}
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

function CategoryTreeNode({
    node,
    depth,
    value,
    expanded,
    onToggle,
    onSelect,
}: {
    node: CategoryNode
    depth: number
    value: number
    expanded: Set<number>
    onToggle: (id: number) => void
    onSelect: (id: number) => void
}) {
    const isExpanded = expanded.has(node.id)
    const isSelected = node.id === value
    const paddingLeft = 8 + depth * 16

    if (node.isLeaf) {
        return (
            <li>
                <button
                    type="button"
                    onClick={() => onSelect(node.id)}
                    style={{ paddingLeft }}
                    className={cn(
                        "flex w-full items-center gap-2 rounded-md py-1.5 pr-2 text-left hover:bg-muted",
                        isSelected ? "font-medium text-foreground" : "text-foreground",
                    )}
                >
                    <span className="flex size-4 shrink-0 items-center justify-center">
                        {isSelected ? <CheckIcon className="size-3.5" /> : null}
                    </span>
                    <span className="truncate">{node.name}</span>
                </button>
            </li>
        )
    }

    return (
        <li>
            <button
                type="button"
                onClick={() => onToggle(node.id)}
                aria-expanded={isExpanded}
                style={{ paddingLeft }}
                className="flex w-full items-center gap-1.5 rounded-md py-1.5 pr-2 text-left font-medium text-muted-foreground hover:bg-muted"
            >
                <ChevronRightIcon
                    className={cn("size-4 shrink-0 transition-transform", isExpanded && "rotate-90")}
                />
                <span className="truncate">{node.name}</span>
            </button>
            {isExpanded ? (
                <ul>
                    {node.children.map((child) => (
                        <CategoryTreeNode
                            key={child.id}
                            node={child}
                            depth={depth + 1}
                            value={value}
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

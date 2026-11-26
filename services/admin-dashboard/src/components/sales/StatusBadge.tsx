import { statusStyle } from "@/lib/sales"

export function StatusBadge({ statuses, value }: { statuses: Parameters<typeof statusStyle>[0]; value: string }) {
    const { label, tone } = statusStyle(statuses, value)
    return (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap ${tone}`}>
            {label}
        </span>
    )
}

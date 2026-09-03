import { create } from "zustand"

type ChannelConnectionsPaginationState = {
    offset: number
    pageSize: number
    setOffset: (offset: number) => void
    setPageSize: (pageSize: number) => void
    reset: () => void
}

const DEFAULT_PAGE_SIZE = 10

export const useChannelConnectionsPaginationStore = create<ChannelConnectionsPaginationState>((set) => ({
    offset: 0,
    pageSize: DEFAULT_PAGE_SIZE,
    setOffset: (offset) => set({ offset: Math.max(0, offset) }),
    setPageSize: (pageSize) => set({ pageSize: pageSize > 0 ? pageSize : DEFAULT_PAGE_SIZE, offset: 0 }),
    reset: () => set({ offset: 0, pageSize: DEFAULT_PAGE_SIZE }),
}))

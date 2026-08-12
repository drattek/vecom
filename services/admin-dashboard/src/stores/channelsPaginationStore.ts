import { create } from 'zustand'

type ChannelsPaginationState = {
  offset: number
  pageSize: number
  setOffset: (offset: number) => void
  setPageSize: (pageSize: number) => void
  reset: () => void
}

const DEFAULT_PAGE_SIZE = 10

export const useChannelsPaginationStore = create<ChannelsPaginationState>((set) => ({
  offset: 0,
  pageSize: DEFAULT_PAGE_SIZE,
  setOffset: (offset) => set({ offset: Math.max(0, offset) }),
  setPageSize: (pageSize) =>
    set({
      pageSize: pageSize > 0 ? pageSize : DEFAULT_PAGE_SIZE,
      offset: 0,
    }),
  reset: () => set({ offset: 0, pageSize: DEFAULT_PAGE_SIZE }),
}))

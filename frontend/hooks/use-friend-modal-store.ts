import { create } from 'zustand';
type FriendTab = 'add' | 'received' | 'friends';
type Store = { open: boolean; tab: FriendTab; show: (tab?: FriendTab) => void; close: () => void; setTab: (tab: FriendTab) => void };
export const useFriendModalStore = create<Store>((set) => ({ open: false, tab: 'add', show: (tab = 'add') => set({ open: true, tab }), close: () => set({ open: false }), setTab: (tab) => set({ tab }) }));

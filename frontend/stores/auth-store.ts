import { create } from 'zustand';
import type { User } from '@/types/user';

type AuthStore = {
  currentUser: User | null;
  setCurrentUser: (user: User) => void;
  clearCurrentUser: () => void;
};

export const useAuthStore = create<AuthStore>((set) => ({
  currentUser: null,
  setCurrentUser: (currentUser) => set({ currentUser }),
  clearCurrentUser: () => set({ currentUser: null }),
}));

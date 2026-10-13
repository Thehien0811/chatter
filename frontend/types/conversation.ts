import type { User } from './user';
export type Conversation = { id: string; title: string; avatarUrl?: string; participant: User; lastMessage?: string; updatedAt: string; unreadCount: number; online?: boolean };

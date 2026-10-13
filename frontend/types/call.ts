import type { User } from './user';
export type CallState = 'idle' | 'incoming' | 'connecting' | 'active';
export type Call = { id: string; conversationId: string; caller: User; video: boolean };

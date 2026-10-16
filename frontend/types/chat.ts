import type { PresenceStatus } from '@/types/presence';

export type ChatAttachment = {
  id: string;
  media_type: string;
  byte_size: number;
  url: string;
};

export type ChatMessage = {
  id: string;
  conversation_id: string;
  sender_id: string;
  body: string;
  created_at: string;
  attachments?: ChatAttachment[];
};

export type ChatConversation = {
  id: string;
  title: string;
  avatar_url?: string;
  participant: {
    id: string;
    username: string;
    display_name: string;
    presence?: PresenceStatus;
    presence_version?: number;
  };
  last_message?: ChatMessage;
  updated_at: string;
  unread_count: number;
};

export type UnreadChatMessage = {
  message_id: string;
  conversation_id: string;
  sender_id: string;
  body: string;
  created_at: string;
};

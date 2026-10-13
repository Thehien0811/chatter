export type MessageStatus = 'sent' | 'delivered' | 'seen';
export type Message = { id: string; conversationId: string; senderId: string; senderName: string; senderAvatar?: string; content: string; createdAt: string; status?: MessageStatus };

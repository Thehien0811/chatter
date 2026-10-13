import { ChatWorkspace } from '@/components/chat/chat-workspace';

export function ChatPanel({ conversationId }: { conversationId?: string }) {
  return <ChatWorkspace initialConversationId={conversationId}/>;
}

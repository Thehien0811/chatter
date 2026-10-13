import { ChatWorkspace } from '@/components/chat/chat-workspace';

export default function ConversationPage({ params }: { params: { conversationId: string } }) {
  return <ChatWorkspace initialConversationId={params.conversationId}/>;
}

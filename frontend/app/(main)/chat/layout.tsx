import type { ReactNode } from 'react';
import { MainChatLayout } from '@/components/chat/main-chat-layout';
import { CallManager } from '@/components/call/call-manager';
export default function ChatLayout({children}:{children:ReactNode}) { return <CallManager><MainChatLayout>{children}</MainChatLayout></CallManager>; }

import type { ReactNode } from 'react';
import { MainChatLayout } from '@/components/chat/main-chat-layout';
export default function ChatLayout({children}:{children:ReactNode}) { return <MainChatLayout>{children}</MainChatLayout>; }

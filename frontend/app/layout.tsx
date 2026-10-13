import type { Metadata } from 'next'; import './globals.css'; import { Providers } from '@/components/providers';
export const metadata: Metadata = { title: 'Chatter', description: 'Stay close to your people.' };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="vi" suppressHydrationWarning><body><Providers>{children}</Providers></body></html>; }

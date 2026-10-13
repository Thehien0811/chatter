'use client';
import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from 'next-themes';
import { useState } from 'react';
import { Toaster, toast } from 'sonner';
export function Providers({ children }: { children: React.ReactNode }) { const [client] = useState(() => new QueryClient({ queryCache: new QueryCache({ onError: () => toast.error('Không tải được dữ liệu. Hãy thử lại.') }), mutationCache: new MutationCache({ onError: () => toast.error('Thao tác thất bại. Hãy thử lại.') }), defaultOptions: { queries: { staleTime: 30_000, retry: 1 } } })); return <ThemeProvider attribute="class" defaultTheme="system" enableSystem><QueryClientProvider client={client}>{children}<Toaster richColors position="top-right" /></QueryClientProvider></ThemeProvider>; }

'use client';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api, setAccessToken } from '@/lib/api-client';
import { getSocket } from '@/lib/websocket-client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAuthStore } from '@/stores/auth-store';
import type { AuthResponse, LoginRequest, RegisterRequest } from '@/types/auth';

const loginSchema = z.object({ username: z.string().min(1, 'Nhập username.'), password: z.string().min(1, 'Nhập mật khẩu.') });
const registerSchema = z.object({
  username: z.string().min(1, 'Nhập username.'),
  email: z.string().email('Email không hợp lệ.'),
  display_name: z.string().min(1, 'Nhập tên hiển thị.'),
  password: z.string().min(1, 'Nhập mật khẩu.'),
  confirm_password: z.string(),
}).refine((values) => values.password === values.confirm_password, { path: ['confirm_password'], message: 'Mật khẩu xác nhận chưa khớp.' });
type AuthFields = {
  username: string;
  password: string;
  display_name?: string;
  email?: string;
  confirm_password?: string;
};

export function AuthForm({ mode }: { mode: 'login' | 'register' }) {
  const router = useRouter();
  const setCurrentUser = useAuthStore((state) => state.setCurrentUser);
  const schema: z.ZodType<AuthFields> = mode === 'login' ? loginSchema : registerSchema;
  const form = useForm<AuthFields>({ resolver: zodResolver(schema) });

  const submit = form.handleSubmit(async (values) => {
    try {
      const result = mode === 'login'
        ? await api<AuthResponse>('/auth/login', { method: 'POST', body: JSON.stringify({ username: values.username, password: values.password } satisfies LoginRequest) })
        : await api<AuthResponse>('/auth/register', { method: 'POST', body: JSON.stringify({ username: values.username, display_name: values.display_name ?? '', email: values.email ?? '', password: values.password } satisfies RegisterRequest) });
      setAccessToken(result.access_token);
      setCurrentUser(result.user);
      getSocket();
      router.push('/chat');
      router.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : mode === 'login' ? 'Không đăng nhập được.' : 'Không tạo được tài khoản.');
    }
  });

  return <div className="w-full max-w-md rounded-3xl border bg-card p-8 shadow-xl sm:p-10">
    <div className="mb-8 text-center"><div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-primary text-xl font-bold text-primary-foreground">c</div><h1 className="mt-5 text-2xl font-bold">{mode === 'login' ? 'Chào mừng bạn trở lại' : 'Tạo tài khoản'}</h1><p className="mt-2 text-sm text-muted-foreground">{mode === 'login' ? 'Đăng nhập để tiếp tục trò chuyện.' : 'Tạo tài khoản để bắt đầu với Chatter.'}</p></div>
    <form className="space-y-4" onSubmit={submit}>
      <label className="block space-y-1.5 text-sm font-medium">Username<Input type="text" autoComplete="username" {...form.register('username')} />{form.formState.errors.username && <span className="text-xs text-destructive">{form.formState.errors.username.message}</span>}</label>
      {mode === 'register' && <>
        <label className="block space-y-1.5 text-sm font-medium">Tên hiển thị<Input type="text" autoComplete="name" {...form.register('display_name')} />{form.formState.errors.display_name && <span className="text-xs text-destructive">{form.formState.errors.display_name.message}</span>}</label>
        <label className="block space-y-1.5 text-sm font-medium">Email<Input type="email" autoComplete="email" {...form.register('email')} />{form.formState.errors.email && <span className="text-xs text-destructive">{form.formState.errors.email.message}</span>}</label>
      </>}
      <label className="block space-y-1.5 text-sm font-medium">Mật khẩu<Input type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} {...form.register('password')}/>{form.formState.errors.password && <span className="text-xs text-destructive">{String(form.formState.errors.password.message)}</span>}</label>
      {mode === 'register' && <label className="block space-y-1.5 text-sm font-medium">Xác nhận mật khẩu<Input type="password" autoComplete="new-password" {...form.register('confirm_password')}/>{form.formState.errors.confirm_password && <span className="text-xs text-destructive">{String(form.formState.errors.confirm_password.message)}</span>}</label>}
      <Button className="w-full" disabled={form.formState.isSubmitting}>{form.formState.isSubmitting ? 'Đang xử lý…' : mode === 'login' ? 'Đăng nhập' : 'Đăng ký'}</Button>
    </form>
    <p className="mt-6 text-center text-sm text-muted-foreground">{mode === 'login' ? 'Chưa có tài khoản? ' : 'Đã có tài khoản? '}<Link className="font-medium text-primary hover:underline" href={mode === 'login' ? '/register' : '/login'}>{mode === 'login' ? 'Đăng ký' : 'Đăng nhập'}</Link></p>
  </div>;
}

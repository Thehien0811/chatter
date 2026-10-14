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

const loginSchema = z.object({ username: z.string().min(1, 'Nhập username.'), password: z.string().min(1, 'Nhập mật khẩu.') });
const registerSchema = z.object({
  username: z.string().min(1, 'Nhập username.'),
  email: z.string().email('Email không hợp lệ.'),
  display_name: z.string().min(1, 'Nhập tên hiển thị.'),
  password: z.string().min(1, 'Nhập mật khẩu.'),
  confirm_password: z.string(),
}).refine((values) => values.password === values.confirm_password, { path: ['confirm_password'], message: 'Mật khẩu xác nhận chưa khớp.' });
type Login = z.infer<typeof loginSchema>;
type Register = z.infer<typeof registerSchema>;
type AuthFields = Login & Partial<Omit<Register, keyof Login>>;
type AuthResponse = { access_token: string; refresh_token: string; token_type: 'Bearer'; expires_in: number; user: { id: string; username: string; display_name: string; email: string; presence: string; status_message: string } };

export function AuthForm({ mode }: { mode: 'login' | 'register' }) {
  const router = useRouter();
  const schema: z.ZodType<AuthFields> = mode === 'login' ? loginSchema : registerSchema;
  const form = useForm<AuthFields>({ resolver: zodResolver(schema) });

  const submit = form.handleSubmit(async (values) => {
    try {
      const result = mode === 'login'
        ? await api<AuthResponse>('/auth/login', { method: 'POST', body: JSON.stringify({ username: values.username, password: values.password }) })
        : await api<AuthResponse>('/auth/register', { method: 'POST', body: JSON.stringify({ username: values.username, display_name: values.display_name ?? '', email: values.email ?? '', password: values.password }) });
      setAccessToken(result.access_token);
      getSocket();
      router.push('/chat');
      router.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : mode === 'login' ? 'Không đăng nhập được.' : 'Không tạo được tài khoản.');
    }
  });

  const fields = mode === 'login'
    ? [{ name: 'username', label: 'Username', type: 'text' }]
    : [{ name: 'username', label: 'Username', type: 'text' }, { name: 'display_name', label: 'Tên hiển thị', type: 'text' }, { name: 'email', label: 'Email', type: 'email' }];

  return <div className="w-full max-w-md rounded-3xl border bg-card p-8 shadow-xl sm:p-10">
    <div className="mb-8 text-center"><div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-primary text-xl font-bold text-primary-foreground">c</div><h1 className="mt-5 text-2xl font-bold">{mode === 'login' ? 'Chào mừng bạn trở lại' : 'Tạo tài khoản'}</h1><p className="mt-2 text-sm text-muted-foreground">{mode === 'login' ? 'Đăng nhập để tiếp tục trò chuyện.' : 'Tạo tài khoản để bắt đầu với Chatter.'}</p></div>
    <form className="space-y-4" onSubmit={submit}>
      {fields.map((field) => <label key={field.name} className="block space-y-1.5 text-sm font-medium">{field.label}<Input type={field.type} autoComplete={field.name === 'username' ? 'username' : undefined} {...form.register(field.name as never)} />{form.formState.errors[field.name as keyof typeof form.formState.errors] && <span className="text-xs text-destructive">{String(form.formState.errors[field.name as keyof typeof form.formState.errors]?.message)}</span>}</label>)}
      <label className="block space-y-1.5 text-sm font-medium">Mật khẩu<Input type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} {...form.register('password')}/>{form.formState.errors.password && <span className="text-xs text-destructive">{String(form.formState.errors.password.message)}</span>}</label>
      {mode === 'register' && <label className="block space-y-1.5 text-sm font-medium">Xác nhận mật khẩu<Input type="password" autoComplete="new-password" {...form.register('confirm_password')}/>{form.formState.errors.confirm_password && <span className="text-xs text-destructive">{String(form.formState.errors.confirm_password.message)}</span>}</label>}
      <Button className="w-full" disabled={form.formState.isSubmitting}>{form.formState.isSubmitting ? 'Đang xử lý…' : mode === 'login' ? 'Đăng nhập' : 'Đăng ký'}</Button>
    </form>
    <p className="mt-6 text-center text-sm text-muted-foreground">{mode === 'login' ? 'Chưa có tài khoản? ' : 'Đã có tài khoản? '}<Link className="font-medium text-primary hover:underline" href={mode === 'login' ? '/register' : '/login'}>{mode === 'login' ? 'Đăng ký' : 'Đăng nhập'}</Link></p>
  </div>;
}

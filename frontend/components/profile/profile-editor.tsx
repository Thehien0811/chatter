'use client';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { toast } from 'sonner';
import { z } from 'zod';
import { api } from '@/lib/api-client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

type Profile = { id: string; display_name: string; email: string; status_message: string };
const schema = z.object({ display_name: z.string().min(1, 'Nhập tên hiển thị.'), email: z.string().email('Email không hợp lệ.'), status_message: z.string() });
type Values = z.infer<typeof schema>;

export function ProfileEditor({ profile }: { profile: Profile }) {
  const [editing, setEditing] = useState(false);
  const queryClient = useQueryClient();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { display_name: profile.display_name, email: profile.email, status_message: profile.status_message || '' } });
  const mutation = useMutation({
    mutationFn: (values: Values) => api<Profile>(`/users/${encodeURIComponent(profile.id)}`, { method: 'PATCH', body: JSON.stringify(values) }),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['profile'] }); setEditing(false); toast.success('Đã cập nhật hồ sơ.'); },
    onError: (error) => toast.error(error instanceof Error ? error.message : 'Không cập nhật được hồ sơ.'),
  });
  const openEditor = () => { form.reset({ display_name: profile.display_name, email: profile.email, status_message: profile.status_message || '' }); setEditing(true); };
  if (!editing) return <Button variant="outline" size="sm" className="mt-4 w-full" onClick={openEditor}>Chỉnh sửa hồ sơ</Button>;
  return <form className="mt-4 space-y-3 border-t pt-4" onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
    <label className="block space-y-1 text-xs font-medium">Tên hiển thị<Input {...form.register('display_name')}/>{form.formState.errors.display_name && <span className="text-destructive">{form.formState.errors.display_name.message}</span>}</label>
    <label className="block space-y-1 text-xs font-medium">Email<Input type="email" {...form.register('email')}/>{form.formState.errors.email && <span className="text-destructive">{form.formState.errors.email.message}</span>}</label>
    <label className="block space-y-1 text-xs font-medium">Trạng thái<textarea rows={2} className="w-full resize-y rounded-md border bg-background px-3 py-2 text-sm font-normal outline-none focus-visible:ring-2 focus-visible:ring-ring" {...form.register('status_message')}/></label>
    <div className="flex gap-2"><Button type="submit" size="sm" className="flex-1" disabled={mutation.isPending}>{mutation.isPending ? 'Đang lưu…' : 'Lưu thay đổi'}</Button><Button type="button" variant="ghost" size="sm" onClick={() => setEditing(false)} disabled={mutation.isPending}>Hủy</Button></div>
  </form>;
}

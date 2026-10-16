import type { PresenceStatus } from '@/types/presence';

export function PresenceIndicator({ status, className = '' }: { status: PresenceStatus; className?: string }) {
  const label = status === 'online' ? 'Đang hoạt động' : status === 'offline' ? 'Ngoại tuyến' : 'Đang kết nối';
  const color = status === 'offline' ? 'bg-presence-offline' : 'bg-presence-online';

  return <span className={`inline-flex items-center gap-1.5 text-xs text-muted-foreground ${className}`}>
    <span aria-hidden="true" className={`h-2 w-2 shrink-0 rounded-full ${color}`} />
    <span>{label}</span>
  </span>;
}

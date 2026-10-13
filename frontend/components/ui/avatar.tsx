import Image from 'next/image';

export function Avatar({ name, src, className = '' }: { name: string; src?: string; className?: string }) { return <span className={`relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-sm font-semibold ${className}`}>{src ? <Image src={src} alt="" fill sizes="40px" unoptimized className="object-cover" /> : name.slice(0, 1).toUpperCase()}</span>; }

'use client';
import { useEffect, useState } from 'react';
import { ImageOff, X } from 'lucide-react';
import Image from 'next/image';
import { Button } from '@/components/ui/button';
import { apiBlob } from '@/lib/api-client';

export function ImageMessage({ mediaId }: { mediaId: string }) {
  const [source, setSource] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    let objectURL = '';
    void apiBlob(`/media/${encodeURIComponent(mediaId)}`)
      .then((blob) => {
        if (!active) return;
        objectURL = URL.createObjectURL(blob);
        setSource(objectURL);
      })
      .catch((reason: unknown) => {
        if (active) setError(reason instanceof Error ? reason.message : 'Không tải được ảnh.');
      });
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [mediaId]);

  if (error) return <div role="alert" className="flex min-h-24 items-center gap-2 rounded-xl bg-muted/70 px-4 py-3 text-xs text-muted-foreground"><ImageOff className="h-4 w-4 shrink-0"/>{error}</div>;
  if (!source) return <div aria-label="Đang tải ảnh" className="h-36 w-48 animate-pulse rounded-xl bg-muted/70"/>;
  return <Image src={source} alt="Ảnh được gửi trong cuộc trò chuyện" width={640} height={480} unoptimized className="max-h-72 max-w-full rounded-xl object-contain"/>;
}

export function ImageDraftPreview({ file, onRemove }: { file: File; onRemove: () => void }) {
  const [source, setSource] = useState('');
  const [previewError, setPreviewError] = useState('');
  useEffect(() => {
    const reader = new FileReader();
    reader.addEventListener('load', () => setSource(typeof reader.result === 'string' ? reader.result : ''));
    reader.addEventListener('error', () => setPreviewError('Không thể xem trước ảnh này.'));
    reader.readAsDataURL(file);
    return () => reader.abort();
  }, [file]);
  return <div className="flex items-center gap-3 rounded-xl border bg-card p-2">
    {source ? <Image src={source} alt="Ảnh đã chọn" width={56} height={56} unoptimized className="h-14 w-14 rounded-lg object-cover"/> : <span className="h-14 w-14 animate-pulse rounded-lg bg-muted"/>}
    <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{file.name}</span><span className="text-xs text-muted-foreground">{previewError || `${(file.size / (1024 * 1024)).toFixed(2)} MB`}</span></span>
    <Button type="button" variant="ghost" size="icon" onClick={onRemove} aria-label="Bỏ ảnh đã chọn"><X className="h-4 w-4"/></Button>
  </div>;
}

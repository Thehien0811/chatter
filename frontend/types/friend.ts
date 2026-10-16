import type { PresenceStatus } from '@/types/presence';

export type Friend = {
  id: string;
  username: string;
  display_name: string;
  presence?: PresenceStatus;
  presence_version?: number;
};

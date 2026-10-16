export type PresenceStatus = 'online' | 'offline' | 'unknown';

export type PresenceSnapshot = {
  user_id: string;
  status: PresenceStatus;
  version: number;
};

export type FriendWithPresence = {
  id: string;
  username: string;
  display_name: string;
  presence: PresenceStatus;
  presence_version: number;
};

import type { User } from '@/types/user';

export type LoginRequest = {
  username: string;
  password: string;
};

export type RegisterRequest = LoginRequest & {
  display_name: string;
  email: string;
};

export type AuthResponse = {
  user: User;
  access_token: string;
  refresh_token: string;
  token_type: 'Bearer';
  expires_in: number;
};

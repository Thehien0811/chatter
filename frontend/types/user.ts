/** User shape returned by the Chatter API Gateway. */
export type User = {
  id: string;
  username: string;
  display_name: string;
  email: string;
  presence: string;
  status_message: string;
};

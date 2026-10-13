# Chatter frontend

Next.js App Router frontend for the Chatter API Gateway.

## Run locally

Set `API_INTERNAL_URL` to the Go API Gateway (default `http://localhost:8080`), then run `npm run dev`. Browser API requests go through the same-origin `/api/backend/*` proxy, which prefixes `/api/v1`, stores access and refresh tokens in HttpOnly cookies, and rotates tokens through the documented JSON refresh endpoint.

## Chat integration

- Registration and login use `{ username, password }`; registration also sends `display_name` and `email`.
- The account dialog reads `GET /profile` and `GET /health`, and profile editing uses `PATCH /users/:id`.
- The proxy supports `POST /auth/refresh` using the refresh token cookie and clears local auth cookies on logout.
- Conversations and message history use the Gateway's `/chat/conversations` API. Sending and receiving messages uses the Gateway WebSocket at `/api/v1/ws/presence`; the browser refreshes its access token before connecting.
- Direct conversation creation requires the recipient's user ID and an existing friendship. There is not yet a friend/user lookup UI or friend request API.

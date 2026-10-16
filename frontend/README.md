# Chatter frontend

Next.js App Router frontend for the Chatter API Gateway.

## Run locally

Set `API_INTERNAL_URL` to the Go API Gateway (default `http://localhost:8080`), then run `npm run dev`. Browser API requests go through the same-origin `/api/backend/*` proxy, which prefixes `/api/v1`, stores access and refresh tokens in HttpOnly cookies, and rotates tokens through the documented JSON refresh endpoint.

## Chat integration

- Registration and login use `{ username, password }`; registration also sends `display_name` and `email`.
- The account dialog reads `GET /profile` and `GET /health`, and profile editing uses `PATCH /users/:id`.
- The proxy supports `POST /auth/refresh` using the refresh token cookie and clears local auth cookies on logout.
- Conversations and message history use the Gateway's `/chat/conversations` API. Sending and receiving messages uses the Gateway WebSocket at `/api/v1/ws/presence`; the browser refreshes its access token before connecting.
- Friend presence is read from the Gateway's Redis-backed snapshot and updated over the existing WebSocket. The Gateway uses Ping/Pong leases; friend lists refresh their batch snapshot periodically as a recovery path for missed events.
- Direct conversation creation requires the recipient's user ID and an existing friendship. Friend search and requests are managed in the friends dialog.
- Image messages upload through the authenticated Gateway and are limited to valid JPEG, PNG, or GIF files up to 10 MB. Images are streamed back only to authenticated conversation members.
- Audio calls use WebRTC peer-to-peer media and the shared WebSocket for call signaling. Configure `NEXT_PUBLIC_STUN_URL`; production deployments also need a reachable TURN service and TURN credentials in `NEXT_PUBLIC_TURN_URL`, `NEXT_PUBLIC_TURN_USER`, and `NEXT_PUBLIC_TURN_PASS`.

import { NextRequest, NextResponse } from 'next/server';
const backend = process.env.API_INTERNAL_URL ?? process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
const apiPrefix = '/api/v1';
type Tokens = { access_token?: string; refresh_token?: string };
function writeTokens(response: NextResponse, tokens: Tokens) {
  const options = { httpOnly: true, secure: process.env.NODE_ENV === 'production', sameSite: 'lax' as const, path: '/' };
  if (tokens.access_token) response.cookies.set('access_token', tokens.access_token, { ...options, maxAge: 60 * 60 * 2 });
  if (tokens.refresh_token) response.cookies.set('refresh_token', tokens.refresh_token, { ...options, maxAge: 60 * 60 * 24 * 7 });
}
async function handle(request: NextRequest, context: { params: { path: string[] } }) {
  const path = context.params.path.join('/'); const url = `${backend.replace(/\/$/, '')}${apiPrefix}/${path}${request.nextUrl.search}`;
  if (path === 'auth/logout') {
    const response = NextResponse.json({ ok: true });
    response.cookies.delete('access_token');
    response.cookies.delete('refresh_token');
    return response;
  }
  let body: ArrayBuffer | string | undefined = ['GET', 'HEAD'].includes(request.method) ? undefined : await request.arrayBuffer();
  const refreshToken = request.cookies.get('refresh_token')?.value;
  if (path === 'auth/refresh' && refreshToken && (!body || (body instanceof ArrayBuffer && body.byteLength === 0))) body = JSON.stringify({ refresh_token: refreshToken });
  const headers = new Headers(); const type = request.headers.get('content-type'); if (type) headers.set('content-type', type);
  if (path === 'auth/refresh' && body) headers.set('content-type', 'application/json');
  const access = request.cookies.get('access_token')?.value; if (access && path !== 'auth/refresh') headers.set('authorization', `Bearer ${access}`);
  const init = { method: request.method, headers, body };
  let upstream = await fetch(url, { ...init, cache: 'no-store' }); let tokens: Tokens = {};
  if (upstream.status === 401 && path !== 'auth/refresh' && request.cookies.get('refresh_token')) {
    const refresh = await fetch(`${backend.replace(/\/$/, '')}${apiPrefix}/auth/refresh`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ refresh_token: request.cookies.get('refresh_token')!.value }), cache: 'no-store' });
    if (refresh.ok) { tokens = await refresh.json() as Tokens; if (tokens.access_token) { headers.set('authorization', `Bearer ${tokens.access_token}`); upstream = await fetch(url, { ...init, headers, cache: 'no-store' }); } }
  } else if (upstream.ok && (path === 'auth/login' || path === 'auth/register' || path === 'auth/refresh')) {
    tokens = await upstream.clone().json() as Tokens;
  }
  const response = new NextResponse(upstream.body, { status: upstream.status, headers: { 'content-type': upstream.headers.get('content-type') ?? 'application/json' } });
  writeTokens(response, tokens);
  if (path === 'auth/logout' || (upstream.status === 401 && path !== 'auth/refresh') || (path === 'auth/refresh' && upstream.status === 401)) { response.cookies.delete('access_token'); response.cookies.delete('refresh_token'); }
  return response;
}
export const GET = handle; export const POST = handle; export const PUT = handle; export const PATCH = handle; export const DELETE = handle;

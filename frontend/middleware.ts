import { NextRequest, NextResponse } from 'next/server';
export function middleware(request:NextRequest) { const authenticated=Boolean(request.cookies.get('access_token')?.value||request.cookies.get('session')?.value);const path=request.nextUrl.pathname;if(path.startsWith('/chat')&&!authenticated)return NextResponse.redirect(new URL('/login',request.url));if((path==='/login'||path==='/register')&&authenticated)return NextResponse.redirect(new URL('/chat',request.url));return NextResponse.next(); }
export const config={matcher:['/chat/:path*','/login','/register']};

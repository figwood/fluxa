import { NextRequest, NextResponse } from 'next/server';

const backendBase = process.env.BACKEND_BASE_URL || 'http://localhost:8090';

async function proxy(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  const target = `${backendBase.replace(/\/$/, '')}/api/v1/${path.join('/')}${new URL(req.url).search}`;
  const body = ['GET', 'HEAD'].includes(req.method) ? undefined : await req.text();
  const cookieToken = req.cookies.get('fluxa_access_token')?.value;
  const authorization = req.headers.get('authorization') || (cookieToken ? `Bearer ${cookieToken}` : '');
  try {
    const res = await fetch(target, {
      method: req.method,
      headers: {
        accept: 'application/json',
        'content-type': req.headers.get('content-type') || 'application/json',
        ...(authorization ? { authorization } : {})
      },
      body,
      cache: 'no-store'
    });
    let text = await res.text();
    let loginToken = '';
    if (path.join('/') === 'auth/login' && res.ok) {
      const payload = JSON.parse(text) as { data?: { access_token?: string } };
      loginToken = payload.data?.access_token || '';
      if (payload.data) delete payload.data.access_token;
      text = JSON.stringify(payload);
    }
    const response = new NextResponse(text, {
      status: res.status,
      headers: { 'content-type': res.headers.get('content-type') || 'application/json' }
    });
    if (loginToken) {
      response.cookies.set('fluxa_access_token', loginToken, {
        httpOnly: true,
        sameSite: 'lax',
        secure: process.env.APP_ENV === 'production',
        path: '/',
        maxAge: Number(process.env.ACCESS_TOKEN_TTL_HOURS || 8) * 60 * 60
      });
    }
    if (path.join('/') === 'auth/logout' || res.status === 401) {
      response.cookies.set('fluxa_access_token', '', { httpOnly: true, sameSite: 'lax', path: '/', maxAge: 0 });
    }
    return response;
  } catch {
    return NextResponse.json({ code: 500, message: 'proxy_error' }, { status: 500 });
  }
}

export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
export const PUT = proxy;
export const DELETE = proxy;

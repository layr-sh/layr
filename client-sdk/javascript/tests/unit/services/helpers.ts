import { vi } from 'vitest';
import { createClient, withServiceAccount } from '../../../src/core/client';

export function createMockServiceContext(
  responseGenerator?: ((url: string, init?: RequestInit) => any) | Record<string, any>
) {
  let lastUrl = '';
  let lastInit: RequestInit | undefined;

  const fetchHandler = vi.fn(async (url: string, init?: RequestInit) => {
    lastUrl = String(url);
    lastInit = init;

    if (responseGenerator) {
      const custom =
        typeof responseGenerator === 'function'
          ? responseGenerator(lastUrl, init)
          : responseGenerator;
      if (custom instanceof Response) return custom;
      if (custom !== undefined) {
        return new Response(JSON.stringify(custom), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
    }

    if (init?.method === 'HEAD') {
      return new Response(null, {
        status: 200,
        headers: { 'Content-Type': 'application/json', 'Content-Length': '100' },
      });
    }

    // Default: if URL indicates an array response, return []
    if (
      lastUrl.endsWith('/passkeys') ||
      lastUrl.endsWith('/deliveries') ||
      lastUrl.endsWith('/indexes') ||
      lastUrl.endsWith('/policies') ||
      lastUrl.includes('/event-hooks') && init?.method === 'GET' && !lastUrl.match(/event-hooks\/[^/]+$/) ||
      lastUrl.includes('/service-accounts') && init?.method === 'GET' && !lastUrl.match(/service-accounts\/[^/]+$/) ||
      lastUrl.includes('/users') && init?.method === 'GET' && !lastUrl.match(/users\/[^/]+$/) ||
      lastUrl.includes('/sessions') && init?.method === 'GET' && !lastUrl.match(/sessions\/[^/]+$/)
    ) {
      return new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response(JSON.stringify({}), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  });

  const client = createClient({
    endpoint: 'https://api.layr.sh',
    publishableKey: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
    fetch: fetchHandler as unknown as typeof fetch,
  });

  const serviceAccountKey = 'fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210';
  const elevate = <T extends object>(svc: T) =>
    withServiceAccount(svc as any, { serviceAccountKey });

  return {
    client,
    fetchHandler,
    elevate,
    serviceAccountKey,
    getLastUrl: () => lastUrl,
    getLastInit: () => lastInit,
  };
}

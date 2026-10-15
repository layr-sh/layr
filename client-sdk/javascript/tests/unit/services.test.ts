import { describe, it, expect, vi } from 'vitest';
import { createClient, withServiceAccount } from '../../src/core/client';
import {
  useAuth,
  useData,
  useFileStorage,
  useImage,
  useFunction,
  useTasks,
  useConsole,
  useCore,
} from '../../src/services';
import { LayrAPIError } from '../../src/core/errors';

describe('Service Methods Unit Tests (SVC-01 to SVC-08)', () => {
  const publishableKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
  const serviceAccountKey = 'fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210';
  const fetchHandler = vi.fn();
  const client = createClient({
    endpoint: 'https://api.layr.sh',
    publishableKey,
    fetch: (input, requestInit) => fetchHandler(input, requestInit),
  });

  it('SVC-01 & SVC-02: Path Parameter Interpolation & Query Param Serialization', async () => {
    let capturedUrl = '';
    fetchHandler.mockImplementation(async (url: string) => {
      capturedUrl = url;
      return new Response(JSON.stringify({ id: 'rec-1', name: 'Item' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });

    const data = useData(client);
    await data.records.get('public', 'items', 'rec-1', 'id,name');

    expect(capturedUrl).toContain('/v1/data/public/items/rec-1');
    expect(capturedUrl).toContain('select=id%2Cname');
  });

  it('SVC-03 & SVC-04: Request Body JSON Serialization & Successful 2xx Deserialization', async () => {
    let capturedBody = '';
    let capturedContentType = '';

    fetchHandler.mockImplementation(async (_url: string, requestOptions: RequestInit) => {
      capturedBody = requestOptions.body as string;
      capturedContentType = (requestOptions.headers as Record<string, string>)['Content-Type'];
      return new Response(
        JSON.stringify({
          access_token: 'new_token_123',
          expires_in: 3600,
          refresh_token: 'ref',
          token_type: 'Bearer',
          user: { id: 'u1', email: 'user@example.com', phone: '', created_at: '', updated_at: '' },
        }),
        {
          status: 201,
          headers: { 'Content-Type': 'application/json' },
        }
      );
    });

    const auth = useAuth(client);
    const response = await auth.sign_up({
      email: 'user@example.com',
      password: 'SecretPassword123!',
      phone: '',
      properties: {},
    });

    expect(capturedContentType).toContain('application/json');
    expect(JSON.parse(capturedBody)).toEqual({
      email: 'user@example.com',
      password: 'SecretPassword123!',
      phone: '',
      properties: {},
    });
    expect(response.accessToken).toBe('new_token_123');
  });

  it('SVC-05: Void / 204 No Content Handling completes without json parsing error', async () => {
    fetchHandler.mockResolvedValue(new Response(null, { status: 204 }));

    const auth = useAuth(client);
    await expect(auth.sign_out()).resolves.toBeUndefined();
  });

  it('SVC-06: RFC 9457 Problem Details Error translates into LayrAPIError', async () => {
    fetchHandler.mockResolvedValue(
      new Response(
        JSON.stringify({
          title: 'Not Found',
          status: 404,
          detail: 'Item not found in schema',
          instance: '/v1/data/public/items/99',
        }),
        {
          status: 404,
          statusText: 'Not Found',
          headers: { 'Content-Type': 'application/problem+json' },
        }
      )
    );

    const data = useData(client);
    await expect(data.records.get('public', 'items', '99')).rejects.toMatchObject({
      name: 'LayrAPIError',
      status: 404,
      title: 'Not Found',
      detail: 'Item not found in schema',
      instance: '/v1/data/public/items/99',
    });
  });

  it('SVC-07: RFC 6749 OAuth 2.0 Error translates into LayrAPIError', async () => {
    fetchHandler.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: 'invalid_grant',
          error_description: 'Bad credentials',
        }),
        {
          status: 401,
          statusText: 'Unauthorized',
          headers: { 'Content-Type': 'application/json' },
        }
      )
    );

    const auth = useAuth(client);
    await expect(
      auth.sign_in({ email: 'user@example.com', password: 'bad', phone: '' })
    ).rejects.toMatchObject({
      name: 'LayrAPIError',
      status: 401,
      error: 'invalid_grant',
      error_description: 'Bad credentials',
    });
  });

  it('SVC-08: Control Plane Elevation Header Enforcement on .cp operations', async () => {
    let capturedHeaders: Record<string, string> = {};

    fetchHandler.mockImplementation(async (_url: string, requestOptions: RequestInit) => {
      capturedHeaders = requestOptions.headers as Record<string, string>;
      return new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });

    const data = useData(client);
    const elevatedData = withServiceAccount(data, { serviceAccountKey });

    await elevatedData.cp.tables.list();
    expect(capturedHeaders['Service-Account-Key']).toBe(serviceAccountKey);
  });

  it('verifies all 8 service hooks instantiate correctly', () => {
    expect(useAuth(client)).toBeDefined();
    expect(useData(client)).toBeDefined();
    expect(useFileStorage(client)).toBeDefined();
    expect(useImage(client)).toBeDefined();
    expect(useFunction(client)).toBeDefined();
    expect(useTasks(client)).toBeDefined();
    expect(useConsole(client)).toBeDefined();
    expect(useCore(client)).toBeDefined();
  });
});

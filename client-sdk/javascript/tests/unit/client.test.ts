import { describe, it, expect, expectTypeOf } from 'vitest';
import { createClient, withServiceAccount, ATTACH_CONTROL_PLANE, type ElevatableService } from '../../src/core/client';
import type { QueryBuilder } from '../../src/extensions/query-builder';
import { useAuth } from '../../src/services/auth';

describe('Client Context & Elevation Unit Tests', () => {
  it('CL-01: Factory Instantiation produces valid LayrClient', () => {
    const publishableKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
    const client = createClient({
      endpoint: 'https://api.layr.sh',
      publishableKey,
    });

    expect(client.endpoint).toBe('https://api.layr.sh');
    expect(client.endpointURL).toBe('https://api.layr.sh');
    expect(client.headers['Publishable-Key']).toBe(publishableKey);
    expect(client._singletons).toBeInstanceOf(Map);
    expect(client.config).toBeDefined();
  });

  it('CL-02: Session Mutability sets Authorization header', () => {
    const client = createClient({ endpoint: 'http://localhost:5297' });
    expect(client.headers['Authorization']).toBeUndefined();

    client.setSession('session_jwt_xyz');
    expect(client.sessionToken).toBe('session_jwt_xyz');
    expect(client.headers['Authorization']).toBe('Bearer session_jwt_xyz');

    client.setSession('Bearer token_with_prefix');
    expect(client.headers['Authorization']).toBe('Bearer token_with_prefix');

    client.setSession('');
    expect(client.headers['Authorization']).toBeUndefined();
  });

  it('CL-03: Session Mutability clears session', () => {
    const client = createClient({
      endpoint: 'http://localhost:5297',
      sessionToken: 'initial_token',
    });
    expect(client.headers['Authorization']).toBe('Bearer initial_token');

    client.clearSession();
    expect(client.sessionToken).toBeUndefined();
    expect(client.headers['Authorization']).toBeUndefined();
  });

  it('CL-04: Elevation attaches .cp control plane namespace', () => {
    const client = createClient({ endpoint: 'http://localhost:5297' });
    const auth = useAuth(client);
    expect((auth as any).cp).toBeUndefined();

    const elevatedAuth = withServiceAccount(auth, {
      serviceAccountKey: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
    });

    expect(elevatedAuth.cp).toBeDefined();
    expect(elevatedAuth.cp.users).toBeDefined();
    expect(elevatedAuth.cp.config).toBeDefined();
  });

  it('CL-05: Elevation preserves base client immutability', () => {
    const client = createClient({ endpoint: 'http://localhost:5297' });
    const auth = useAuth(client);
    withServiceAccount(auth, {
      serviceAccountKey: 'secret_key',
    });

    expect(client.headers['Service-Account-Key']).toBeUndefined();
    expect((auth as any).cp).toBeUndefined();
  });

  it('CL-06: Elevation Guard throws on empty serviceAccountKey', () => {
    const client = createClient({ endpoint: 'http://localhost:5297' });
    const auth = useAuth(client);

    expect(() => withServiceAccount(auth, { serviceAccountKey: '' })).toThrow(
      'serviceAccountKey is required'
    );
    expect(() => withServiceAccount(auth, { serviceAccountKey: '   ' })).toThrow(
      'serviceAccountKey is required'
    );
    expect(() => withServiceAccount(auth, null as any)).toThrow(
      'serviceAccountKey is required'
    );
  });

  it('CL-07: Elevation Guard throws on invalid service', () => {
    expect(() =>
      withServiceAccount({} as any, { serviceAccountKey: 'valid_key' })
    ).toThrow('Provided service is not a valid Layr service client');
  });

  it('CL-08: Elevation on service without ATTACH_CONTROL_PLANE yields undefined cp', () => {
    const client = createClient({ endpoint: 'http://localhost:5297' });
    const simpleService: ElevatableService<unknown> = { client };
    const elevated = withServiceAccount(simpleService, { serviceAccountKey: 'key' });
    expect(elevated.cp).toBeUndefined();
  });

  describe('CL-09: Typed client.from and client.schema', () => {
    interface MyDatabase {
      public: {
        Tables: {
          todos: {
            Row: { id: number; title: string; completed: boolean };
            Insert: { title: string; completed?: boolean };
            Update: { title?: string; completed?: boolean };
          };
          users: {
            Row: { id: string; name: string };
            Insert: { id?: string; name: string };
            Update: { name?: string };
          };
        };
      };
      audit: {
        Tables: {
          logs: {
            Row: { logId: string; message: string; timestamp: number };
            Insert: { message: string };
            Update: { message?: string };
          };
        };
      };
    }

    it('instantiates client.from and client.schema with schema routing', () => {
      const client = createClient<MyDatabase>({ endpoint: 'http://localhost:5297' });

      const todosBuilder = client.from('todos');
      expect(todosBuilder).toBeDefined();
      expectTypeOf(todosBuilder).toEqualTypeOf<
        QueryBuilder<
          { id: number; title: string; completed: boolean },
          { title: string; completed?: boolean },
          { title?: string; completed?: boolean }
        >
      >();

      const auditLogsBuilder = client.schema('audit').from('logs');
      expect(auditLogsBuilder).toBeDefined();
      expectTypeOf(auditLogsBuilder).toEqualTypeOf<
        QueryBuilder<
          { logId: string; message: string; timestamp: number },
          { message: string },
          { message?: string }
        >
      >();

      interface AdHocPost {
        postId: string;
        body: string;
      }
      const customBuilder = client.from<AdHocPost>('posts');
      expect(customBuilder).toBeDefined();
      expectTypeOf(customBuilder).toEqualTypeOf<
        QueryBuilder<AdHocPost, Partial<AdHocPost>, Partial<AdHocPost>>
      >();
    });
  });
});

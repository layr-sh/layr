import { describe, it, expectTypeOf } from 'vitest';
import { createClient, type LayrClient, type SchemaClient } from '../../src/core/client';
import type { GenericDatabase } from '../../src/core/database';
import { useData, type DataService } from '../../src/services/data';
import { useAuth, type AuthService } from '../../src/services/auth';
import { useFileStorage, type FileStorageService } from '../../src/services/file-storage';
import { useFunction, type FunctionService } from '../../src/services/function';
import { useRealtime, type RealtimeClient } from '../../src/extensions/realtime';
import type { QueryBuilder } from '../../src/extensions/query-builder';

interface MockDatabase {
  public: {
    Tables: {
      users: {
        Row: { id: string; email: string };
        Insert: { email: string };
        Update: { email?: string };
      };
    };
  };
  analytics: {
    Tables: {
      events: {
        Row: { eventId: string; type: string; timestamp: number };
        Insert: { type: string; timestamp: number };
        Update: { type?: string };
      };
    };
  };
}

describe('LayrClient & Service Wrappers Type Safety', () => {
  describe('1. Client Instantiation & Configuration Typing', () => {
    it('creates typed client instance with endpoint', () => {
      const client = createClient({
        endpoint: 'https://api.layr.sh',
        publishableKey: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
        sessionToken: 'tok_abc',
        maxRetries: 3,
        retryDelayMs: 200,
        timeoutMs: 5000,
        headers: { 'Custom-Header': 'header' },
      });

      expectTypeOf(client).toEqualTypeOf<LayrClient>();
      expectTypeOf(client.endpoint).toEqualTypeOf<string>();
      expectTypeOf(client.sessionToken).toEqualTypeOf<string | undefined>();
    });

    it('rejects missing or invalid client options', () => {
      // @ts-expect-error - missing required endpoint option
      () => createClient({});

      // @ts-expect-error - endpoint must be string, not number
      () => createClient({ endpoint: 12345 });

      // @ts-expect-error - maxRetries must be number, not string
      () => createClient({ endpoint: 'https://api.layr.sh', maxRetries: 'five' });
    });
  });

  describe('2. Session Management Typing', () => {
    const client = createClient({ endpoint: 'https://api.layr.sh' });

    it('manages session token with strict string types', () => {
      client.setSession('session_jwt_123');
      expectTypeOf(client.setSession).parameter(0).toEqualTypeOf<string>();

      client.clearSession();
      expectTypeOf(client.clearSession).returns.toEqualTypeOf<void>();

      // @ts-expect-error - token must be string
      () => client.setSession(999);
    });
  });

  describe('3. SchemaClient & Multi-Schema Navigation', () => {
    const client = createClient({ endpoint: 'https://api.layr.sh' });
    const data = useData<MockDatabase>(client);

    it('returns strongly-typed SchemaClient instances', () => {
      const publicSchema = data.schema('public');
      expectTypeOf(publicSchema).toEqualTypeOf<SchemaClient<MockDatabase, 'public'>>();

      const analyticsSchema = data.schema('analytics');
      expectTypeOf(analyticsSchema).toEqualTypeOf<SchemaClient<MockDatabase, 'analytics'>>();
    });

    it('infers correct QueryBuilder types from SchemaClient', () => {
      const userQb = data.schema('public').from('users');
      expectTypeOf(userQb).toEqualTypeOf<
        QueryBuilder<
          MockDatabase['public']['Tables']['users']['Row'],
          MockDatabase['public']['Tables']['users']['Insert'],
          MockDatabase['public']['Tables']['users']['Update']
        >
      >();

      const eventQb = data.schema('analytics').from('events');
      expectTypeOf(eventQb).toEqualTypeOf<
        QueryBuilder<
          MockDatabase['analytics']['Tables']['events']['Row'],
          MockDatabase['analytics']['Tables']['events']['Insert'],
          MockDatabase['analytics']['Tables']['events']['Update']
        >
      >();
    });

    it('rejects invalid schema names on typed data service', () => {
      // @ts-expect-error - 'nonexistent_schema' is not a schema in MockDatabase
      () => data.schema('nonexistent_schema');
    });

    it('rejects .from or .schema on base LayrClient', () => {
      // @ts-expect-error - 'from' does not exist on LayrClient
      () => client.from('users');

      // @ts-expect-error - 'schema' does not exist on LayrClient
      () => client.schema('public');
    });

    it('allows any schema and table name on untyped data service', () => {
      const untyped = createClient({ endpoint: 'https://api.layr.sh' });
      const untypedData = useData(untyped);
      const dynamicSchema = untypedData.schema('any_dynamic_schema');
      const dynamicQb = dynamicSchema.from('any_dynamic_table');

      expectTypeOf(dynamicQb).toEqualTypeOf<
        QueryBuilder<Record<string, unknown>, Partial<Record<string, unknown>>, Partial<Record<string, unknown>>>
      >();
    });
  });

  describe('4. Service Wrapper Singletons & Type Preservation', () => {
    const client = createClient({ endpoint: 'https://api.layr.sh' });

    it('preserves database type in useData service wrapper', () => {
      const data = useData<MockDatabase>(client);
      expectTypeOf(data).toEqualTypeOf<DataService<MockDatabase>>();

      const qb = data.from('users');
      expectTypeOf(qb).toEqualTypeOf<
        QueryBuilder<
          MockDatabase['public']['Tables']['users']['Row'],
          MockDatabase['public']['Tables']['users']['Insert'],
          MockDatabase['public']['Tables']['users']['Update']
        >
      >();
    });

    it('types standard singleton service accessors', () => {
      expectTypeOf(useFileStorage(client)).toEqualTypeOf<FileStorageService>();
      expectTypeOf(useRealtime(client)).toEqualTypeOf<RealtimeClient>();
      expectTypeOf(useAuth(client)).toEqualTypeOf<AuthService>();
      expectTypeOf(useFunction(client)).toEqualTypeOf<FunctionService>();
    });
  });
});

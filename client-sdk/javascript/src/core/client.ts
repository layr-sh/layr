import { Configuration } from '../../generated/runtime';
import {
  buildHeaders,
  createFetchWithRetry,
  normalizeEndpointURL,
  type RetryOptions,
} from './transport';
import type {
  GenericDatabase,
  ResolveInsert,
  ResolveRow,
  ResolveTable,
  ResolveUpdate,
} from './database';
import { QueryBuilder } from '../extensions/query-builder';

export interface ClientOptions extends RetryOptions {
  endpoint: string;
  publishableKey?: string;
  sessionToken?: string;
  headers?: Record<string, string>;
}

export interface SchemaClient<Database = GenericDatabase, SchemaName extends string = 'public'> {
  from<
    TableName extends (Database extends Record<string, any>
      ? SchemaName extends keyof Database
        ? 'Tables' extends keyof Database[SchemaName]
          ? keyof Database[SchemaName]['Tables'] & string
          : string
        : string
      : string)
  >(
    table: TableName
  ): QueryBuilder<
    ResolveRow<ResolveTable<Database, SchemaName, TableName>>,
    ResolveInsert<ResolveTable<Database, SchemaName, TableName>>,
    ResolveUpdate<ResolveTable<Database, SchemaName, TableName>>
  >;

  from<
    CustomRow extends Record<string, any> = never,
    CustomInsert = Partial<CustomRow>,
    CustomUpdate = Partial<CustomRow>
  >(
    table: [CustomRow] extends [never] ? never : string
  ): QueryBuilder<CustomRow, CustomInsert, CustomUpdate>;
}

export interface LayrClient<Database = GenericDatabase> {
  readonly endpoint: string;
  readonly options: ClientOptions;
  readonly config: Configuration;
  readonly headers: Record<string, string>;
  sessionToken?: string;
  setSession(token: string): void;
  clearSession(): void;
  _singletons: Map<string, unknown>;

  from<
    TableName extends (Database extends Record<string, any>
      ? 'public' extends keyof Database
        ? 'Tables' extends keyof Database['public']
          ? keyof Database['public']['Tables'] & string
          : string
        : string
      : string)
  >(
    table: TableName
  ): QueryBuilder<
    ResolveRow<ResolveTable<Database, 'public', TableName>>,
    ResolveInsert<ResolveTable<Database, 'public', TableName>>,
    ResolveUpdate<ResolveTable<Database, 'public', TableName>>
  >;

  from<
    CustomRow extends Record<string, any> = never,
    CustomInsert = Partial<CustomRow>,
    CustomUpdate = Partial<CustomRow>
  >(
    table: [CustomRow] extends [never] ? never : string
  ): QueryBuilder<CustomRow, CustomInsert, CustomUpdate>;

  schema<SchemaName extends (Database extends Record<string, any> ? keyof Database & string : string)>(
    schema: SchemaName
  ): SchemaClient<Database, SchemaName>;
}

export function createClient<Database = GenericDatabase>(options: ClientOptions): LayrClient<Database> {
  const normalizedURL = normalizeEndpointURL(options?.endpoint);
  const activeHeaders = buildHeaders(options);
  const fetchWithRetry = createFetchWithRetry(options);

  const config = new Configuration({
    basePath: normalizedURL,
    headers: activeHeaders,
    fetchApi: fetchWithRetry,
  });

  const client: LayrClient<Database> = {
    endpoint: normalizedURL,
    options,
    config,
    headers: activeHeaders,
    sessionToken: options?.sessionToken,
    _singletons: new Map<string, unknown>(),

    setSession(token: string) {
      this.sessionToken = token;
      const cleanToken = token.replace(/^Bearer\s+/i, '').trim();
      if (cleanToken) {
        this.headers['Authorization'] = `Bearer ${cleanToken}`;
      } else {
        delete this.headers['Authorization'];
      }
    },

    clearSession() {
      delete this.sessionToken;
      delete this.headers['Authorization'];
    },

    from(table: string): any {
      return new QueryBuilder(this as any, table);
    },

    schema(schemaName: string): any {
      return {
        from: (tableName: string) => new QueryBuilder(this as any, `${schemaName}.${tableName}`),
      };
    },
  };

  return client;
}

export const ATTACH_CONTROL_PLANE = Symbol('attachControlPlane');

export interface ElevatableService<TCP = unknown, Database = any> {
  client: LayrClient<Database>;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => TCP;
}

export type Elevated<T> = T extends ElevatableService<infer TCP, any>
  ? T & { cp: TCP }
  : T;

export interface WithServiceAccountOptions {
  serviceAccountKey?: string;
}

export function withServiceAccount<T extends ElevatableService<unknown, any>>(
  service: T,
  options?: WithServiceAccountOptions
): Elevated<T> {
  const elevatable = service as ElevatableService<unknown>;
  if (!elevatable || !elevatable.client) {
    throw new Error('Provided service is not a valid Layr service client');
  }

  const elevatedHeaders: Record<string, string> = {
    ...elevatable.client.headers,
  };

  const key = options?.serviceAccountKey?.trim();
  if (key) {
    elevatedHeaders['Service-Account-Key'] = key;
  }

  const elevatedConfig = new Configuration({
    basePath: elevatable.client.endpoint,
    headers: elevatedHeaders,
    fetchApi: elevatable.client.config.fetchApi,
  });

  const cp = typeof elevatable[ATTACH_CONTROL_PLANE] === 'function'
    ? elevatable[ATTACH_CONTROL_PLANE](elevatedConfig)
    : undefined;

  return Object.assign(Object.create(Object.getPrototypeOf(service)), service, {
    cp,
  }) as Elevated<T>;
}

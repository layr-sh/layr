import type { Configuration, RequestOpts } from '../../generated/runtime';
import {
  CacheApi,
  ConfigApi,
  GraphQLApi,
  KVApi,
  RecordsApi,
  RPCApi,
  TablesApi,
  TablesColumnsApi,
  TablesIndexesApi,
  TablesPoliciesApi,
  TablesRLSApi,
  type DataRecordsListRequest,
} from '../../generated/apis';
import type {
  Column,
  Config,
  CreateColumnResponse,
  CreateIndexInput,
  CreateIndexResponse,
  CreatePolicyInput,
  CreatePolicyResponse,
  CreateRecordInput,
  CreateRecordResponse,
  CreateTableInput,
  CreateTableResponse,
  DeleteColumnResponse,
  DeleteIndexResponse,
  DeletePolicyResponse,
  DeleteTableResponse,
  ExecuteGraphQLInput,
  ExecuteGraphQLResponse,
  FlushCacheResponse,
  GetKVResponse,
  GetMultipleKVInput,
  GetMultipleKVResponse,
  GetRecordResponse,
  IncrementKVInput,
  IncrementKVResponse,
  InvalidateCacheInput,
  InvalidateCacheResponse,
  ListIndexesResponse,
  ListPoliciesResponse,
  ListRecordsResponse,
  ListTablesResponse,
  SetKVInput,
  SetKVResponse,
  SetMultipleKVInput,
  SetMultipleKVResponse,
  Table,
  ToggleRLSInput,
  ToggleRLSResponse,
  TouchKVInput,
  TouchKVResponse,
  UpdateColumnInput,
  UpdateColumnResponse,
  UpdateRecordInput,
  UpdateRecordResponse,
} from '../../generated/models';
import { ATTACH_CONTROL_PLANE, type ElevatableService, type LayrClient } from '../core/client';
import type {
  GenericDatabase,
  ResolveInsert,
  ResolveRow,
  ResolveTable,
  ResolveUpdate,
} from '../core/database';
import { handleApiCall } from '../core/transport';
import { QueryBuilder } from '../extensions/query-builder';
import { RealtimeChannel, RealtimeManager, type SubscribeOptions } from '../extensions/realtime';

export interface DataControlPlane {
  cache: {
    flush: () => Promise<FlushCacheResponse>;
    invalidate: (input: InvalidateCacheInput) => Promise<InvalidateCacheResponse>;
  };
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
  tables: {
    create: (input: CreateTableInput) => Promise<CreateTableResponse>;
    delete: (schemaName: string, tableName: string) => Promise<DeleteTableResponse>;
    get: (schemaName: string, tableName: string) => Promise<Table>;
    list: () => Promise<ListTablesResponse>;
    columns: {
      create: (schemaName: string, tableName: string, column: Column) => Promise<CreateColumnResponse>;
      delete: (schemaName: string, tableName: string, columnName: string) => Promise<DeleteColumnResponse>;
      update: (schemaName: string, tableName: string, columnName: string, input: UpdateColumnInput) => Promise<UpdateColumnResponse>;
    };
    indexes: {
      create: (schemaName: string, tableName: string, input: CreateIndexInput) => Promise<CreateIndexResponse>;
      delete: (schemaName: string, tableName: string, indexName: string) => Promise<DeleteIndexResponse>;
      list: (schemaName: string, tableName: string) => Promise<ListIndexesResponse>;
    };
    policies: {
      create: (schemaName: string, tableName: string, input: CreatePolicyInput) => Promise<CreatePolicyResponse>;
      delete: (schemaName: string, tableName: string, policyName: string) => Promise<DeletePolicyResponse>;
      list: (schemaName: string, tableName: string) => Promise<ListPoliciesResponse>;
    };
    rls: {
      update: (schemaName: string, tableName: string, input: ToggleRLSInput, action?: string, force?: boolean) => Promise<ToggleRLSResponse>;
    };
  };
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

export type ColumnFilters<T> = {
  [K in keyof T]?: string | number | boolean;
};

export type ListRecordsParams<T = Record<string, string | number | boolean>> = Omit<
  DataRecordsListRequest,
  'schemaName' | 'tableName'
> & ColumnFilters<T>;

export interface DataService<Database = GenericDatabase> extends ElevatableService<DataControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => DataControlPlane;

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

  subscribe: <T = Record<string, unknown>>(table: string, options?: SubscribeOptions) => RealtimeChannel<T>;

  graphql: (input: ExecuteGraphQLInput) => Promise<ExecuteGraphQLResponse>;

  kv: {
    delete: (key: string) => Promise<void>;
    get: (key: string) => Promise<GetKVResponse>;
    increment: (input: IncrementKVInput) => Promise<IncrementKVResponse>;
    mget: (input: GetMultipleKVInput) => Promise<GetMultipleKVResponse>;
    mset: (input: SetMultipleKVInput) => Promise<SetMultipleKVResponse>;
    put: (key: string, body: string | object, ttl?: number) => Promise<SetKVResponse>;
    set: (key: string, input: SetKVInput) => Promise<SetKVResponse>;
    touch: (key: string, input: TouchKVInput) => Promise<TouchKVResponse>;
  };

  records: {
    create: <T = Record<string, unknown>>(
      schemaName: string,
      tableName: string,
      input: unknown,
      onConflict?: string
    ) => Promise<T>;
    delete: (schemaName: string, tableName: string, recordId: string) => Promise<void>;
    get: <T = Record<string, unknown>>(
      schemaName: string,
      tableName: string,
      recordId: string,
      select?: string
    ) => Promise<T>;
    list: <T = Record<string, unknown>, F = T>(
      schemaName: string,
      tableName: string,
      params?: ListRecordsParams<F>
    ) => Promise<T[]>;
    put: <T = Record<string, unknown>>(
      schemaName: string,
      tableName: string,
      recordId: string,
      input: unknown
    ) => Promise<T>;
    update: <T = Record<string, unknown>>(
      schemaName: string,
      tableName: string,
      recordId: string,
      input: unknown
    ) => Promise<T>;
  };

  rpc: {
    mutation: (schemaName: string, functionName: string, body: object, invalidateTables?: string) => Promise<object>;
    query: (schemaName: string, functionName: string, invalidateTables?: string) => Promise<object>;
  };
}

export function useData<Database = GenericDatabase>(client: LayrClient): DataService<Database> {
  const config = client.config;

  const service: DataService<Database> = {
    client,

    from: (table: string): any => new QueryBuilder(client, table),

    schema: (schemaName: string): any => ({
      from: (tableName: string) => new QueryBuilder(client, `${schemaName}.${tableName}`),
    }),

    subscribe: <T = Record<string, unknown>>(table: string, options?: SubscribeOptions) => {
      let rtManager = client._singletons.get('realtimeManager') as RealtimeManager | undefined;
      if (!rtManager) {
        rtManager = new RealtimeManager(client);
        client._singletons.set('realtimeManager', rtManager);
      }
      return rtManager.subscribe<T>(table, options);
    },

    graphql: (input: ExecuteGraphQLInput) =>
      handleApiCall(() =>
        new GraphQLApi(config).dataGraphql({ executeGraphQLInput: input })
      ),

    kv: {
      delete: (key: string) => handleApiCall(() => new KVApi(config).dataKvDelete({ key })),
      get: (key: string) => handleApiCall(() => new KVApi(config).dataKvGet({ key })),
      increment: (input: IncrementKVInput) =>
        handleApiCall(() => new KVApi(config).dataKvIncrement({ incrementKVInput: input })),
      mget: (input: GetMultipleKVInput) =>
        handleApiCall(() => new KVApi(config).dataKvMget({ getMultipleKVInput: input })),
      mset: (input: SetMultipleKVInput) =>
        handleApiCall(() => new KVApi(config).dataKvMset({ setMultipleKVInput: input })),
      put: (key: string, body: string | object, ttl?: number) =>
        handleApiCall(() =>
          new KVApi(config).dataKvPut({
            key,
            body: typeof body === 'string' ? body : JSON.stringify(body),
            ttl,
          })
        ),
      set: (key: string, input: SetKVInput) =>
        handleApiCall(() => new KVApi(config).dataKvSet({ key, setKVInput: input })),
      touch: (key: string, input: TouchKVInput) =>
        handleApiCall(() => new KVApi(config).dataKvTouch({ key, touchKVInput: input })),
    },

    records: {
      create: <T = Record<string, unknown>>(
        schemaName: string,
        tableName: string,
        input: unknown,
        onConflict?: string
      ) =>
        handleApiCall(async () => {
          const api = new RecordsApi(config);
          const urlPath = `/v1/data/${encodeURIComponent(schemaName)}/${encodeURIComponent(tableName)}`;
          const queryParameters: Record<string, string> = {};
          if (onConflict) {
            queryParameters['on_conflict'] = onConflict;
          }
          const requestOptions: RequestOpts = {
            path: urlPath,
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            query: queryParameters,
            body: input,
          };
          const response = await (api as unknown as { request(opts: RequestOpts): Promise<Response> }).request(requestOptions);
          if (response.status === 204) {
            return undefined as unknown as T;
          }
          const json = await response.json();
          return (json?.data !== undefined && !Array.isArray(json) && !Array.isArray(input) ? json.data : json) as T;
        }),
      delete: (schemaName: string, tableName: string, recordId: string) =>
        handleApiCall(() => new RecordsApi(config).dataRecordsDelete({ schemaName, tableName, recordId })),
      get: <T = Record<string, unknown>>(
        schemaName: string,
        tableName: string,
        recordId: string,
        select?: string
      ) =>
        handleApiCall(async () => {
          const api = new RecordsApi(config);
          const requestOptions = await api.dataRecordsGetRequestOpts({
            schemaName,
            tableName,
            recordId,
            select,
          });
          const response = await (api as unknown as { request(opts: RequestOpts): Promise<Response> }).request(requestOptions);
          const json = await response.json();
          return (json?.data !== undefined ? json.data : json) as T;
        }),
      list: <T = Record<string, unknown>, F = T>(
        schemaName: string,
        tableName: string,
        params?: ListRecordsParams<F>
      ) =>
        handleApiCall(async () => {
          const api = new RecordsApi(config);
          const requestOptions = await api.dataRecordsListRequestOpts({
            schemaName,
            tableName,
            limit: params?.limit,
            offset: params?.offset,
            select: params?.select,
            order: params?.order,
            cacheTtl: params?.cacheTtl,
            cacheKeySuffix: params?.cacheKeySuffix,
            cacheKey: params?.cacheKey,
          });

          if (params) {
            const standardKeys = new Set([
              'limit',
              'offset',
              'select',
              'order',
              'cacheTtl',
              'cacheKeySuffix',
              'cacheKey',
            ]);
            const query = (requestOptions.query ??= {});
            for (const [key, value] of Object.entries(params)) {
              if (!standardKeys.has(key) && value !== undefined && value !== null) {
                query[key] = value;
              }
            }
          }

          const response = await (api as unknown as { request(opts: RequestOpts): Promise<Response> }).request(requestOptions);
          const json = await response.json();
          return (Array.isArray(json) ? json : (json?.data ?? [json])) as T[];
        }),
      put: <T = Record<string, unknown>>(
        schemaName: string,
        tableName: string,
        recordId: string,
        input: unknown
      ) =>
        handleApiCall(async () => {
          const api = new RecordsApi(config);
          const urlPath = `/v1/data/${encodeURIComponent(schemaName)}/${encodeURIComponent(tableName)}/${encodeURIComponent(recordId)}`;
          const requestOptions: RequestOpts = {
            path: urlPath,
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: input,
          };
          const response = await (api as unknown as { request(opts: RequestOpts): Promise<Response> }).request(requestOptions);
          if (response.status === 204) {
            return undefined as unknown as T;
          }
          const json = await response.json();
          return (json?.data !== undefined ? json.data : json) as T;
        }),
      update: <T = Record<string, unknown>>(
        schemaName: string,
        tableName: string,
        recordId: string,
        input: unknown
      ) =>
        handleApiCall(async () => {
          const api = new RecordsApi(config);
          const urlPath = `/v1/data/${encodeURIComponent(schemaName)}/${encodeURIComponent(tableName)}/${encodeURIComponent(recordId)}`;
          const requestOptions: RequestOpts = {
            path: urlPath,
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: input,
          };
          const response = await (api as unknown as { request(opts: RequestOpts): Promise<Response> }).request(requestOptions);
          if (response.status === 204) {
            return undefined as unknown as T;
          }
          const json = await response.json();
          return (json?.data !== undefined ? json.data : json) as T;
        }),
    },

    rpc: {
      mutation: (schemaName: string, functionName: string, body: object, invalidateTables?: string) =>
        handleApiCall(async () => {
          const response = await new RPCApi(config).dataRpcMutationRaw({
            schemaName,
            functionName,
            body,
            invalidateTables,
          });
          return (await response.value()) as object;
        }),
      query: (schemaName: string, functionName: string, invalidateTables?: string) =>
        handleApiCall(async () => {
          const response = await new RPCApi(config).dataRpcQueryRaw({
            schemaName,
            functionName,
            invalidateTables,
          });
          return (await response.value()) as object;
        }),
    },

    [ATTACH_CONTROL_PLANE]: (elevatedConfig: Configuration): DataControlPlane => ({
      cache: {
        flush: () => handleApiCall(() => new CacheApi(elevatedConfig).dataCacheFlush({ body: {} })),
        invalidate: (input: InvalidateCacheInput) =>
          handleApiCall(() =>
            new CacheApi(elevatedConfig).dataCacheInvalidate({ invalidateCacheInput: input })
          ),
      },
      config: {
        get: () => handleApiCall(() => new ConfigApi(elevatedConfig).dataConfigGet()),
        update: (input: Config) =>
          handleApiCall(() => new ConfigApi(elevatedConfig).dataConfigUpdate({ config: input })),
      },
      tables: {
        create: (input: CreateTableInput) =>
          handleApiCall(() =>
            new TablesApi(elevatedConfig).dataTablesCreate({ createTableInput: input })
          ),
        delete: (schemaName: string, tableName: string) =>
          handleApiCall(() => new TablesApi(elevatedConfig).dataTablesDelete({ schemaName, tableName })),
        get: (schemaName: string, tableName: string) =>
          handleApiCall(() => new TablesApi(elevatedConfig).dataTablesGet({ schemaName, tableName })),
        list: () => handleApiCall(() => new TablesApi(elevatedConfig).dataTablesList()),
        columns: {
          create: (schemaName: string, tableName: string, column: Column) =>
            handleApiCall(() =>
              new TablesColumnsApi(elevatedConfig).dataTablesColumnsCreate({
                schemaName,
                tableName,
                column,
              })
            ),
          delete: (schemaName: string, tableName: string, columnName: string) =>
            handleApiCall(() =>
              new TablesColumnsApi(elevatedConfig).dataTablesColumnsDelete({
                schemaName,
                tableName,
                columnName,
              })
            ),
          update: (schemaName: string, tableName: string, columnName: string, input: UpdateColumnInput) =>
            handleApiCall(() =>
              new TablesColumnsApi(elevatedConfig).dataTablesColumnsUpdate({
                schemaName,
                tableName,
                columnName,
                updateColumnInput: input,
              })
            ),
        },
        indexes: {
          create: (schemaName: string, tableName: string, input: CreateIndexInput) =>
            handleApiCall(() =>
              new TablesIndexesApi(elevatedConfig).dataTablesIndexesCreate({
                schemaName,
                tableName,
                createIndexInput: input,
              })
            ),
          delete: (schemaName: string, tableName: string, indexName: string) =>
            handleApiCall(() =>
              new TablesIndexesApi(elevatedConfig).dataTablesIndexesDelete({
                schemaName,
                tableName,
                indexName,
              })
            ),
          list: (schemaName: string, tableName: string) =>
            handleApiCall(() =>
              new TablesIndexesApi(elevatedConfig).dataTablesIndexesList({ schemaName, tableName })
            ),
        },
        policies: {
          create: (schemaName: string, tableName: string, input: CreatePolicyInput) =>
            handleApiCall(() =>
              new TablesPoliciesApi(elevatedConfig).dataTablesPoliciesCreate({
                schemaName,
                tableName,
                createPolicyInput: input,
              })
            ),
          delete: (schemaName: string, tableName: string, policyName: string) =>
            handleApiCall(() =>
              new TablesPoliciesApi(elevatedConfig).dataTablesPoliciesDelete({
                schemaName,
                tableName,
                policyName,
              })
            ),
          list: (schemaName: string, tableName: string) =>
            handleApiCall(() =>
              new TablesPoliciesApi(elevatedConfig).dataTablesPoliciesList({ schemaName, tableName })
            ),
        },
        rls: {
          update: (schemaName: string, tableName: string, input: ToggleRLSInput, action?: string, force?: boolean) =>
            handleApiCall(() =>
              new TablesRLSApi(elevatedConfig).dataTablesRlsUpdate({
                schemaName,
                tableName,
                toggleRLSInput: input,
                action,
                force,
              })
            ),
        },
      },
    }),
  };

  return service;
}

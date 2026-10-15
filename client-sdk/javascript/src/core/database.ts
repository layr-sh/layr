export interface GenericTable {
  Row: Record<string, unknown>;
  Insert?: Record<string, unknown>;
  Update?: Record<string, unknown>;
}

export interface GenericSchema {
  Tables: Record<string, GenericTable>;
}

export interface GenericDatabase {
  public: GenericSchema;
  [schema: string]: GenericSchema;
}

export type ResolveTable<
  DB,
  SchemaName extends string,
  TableName extends string,
  AdHoc = void
> = [AdHoc] extends [void]
  ? DB extends Record<string, any>
    ? SchemaName extends keyof DB
      ? 'Tables' extends keyof DB[SchemaName]
        ? TableName extends keyof DB[SchemaName]['Tables']
          ? DB[SchemaName]['Tables'][TableName]
          : GenericTable
        : GenericTable
      : GenericTable
    : GenericTable
  : AdHoc extends Record<string, any>
  ? { Row: AdHoc; Insert: Partial<AdHoc>; Update: Partial<AdHoc> }
  : GenericTable;

export type ResolveRow<T> = T extends { Row: infer R } ? R : Record<string, unknown>;
export type ResolveInsert<T> = T extends { Insert: infer I } ? I : Partial<ResolveRow<T>>;
export type ResolveUpdate<T> = T extends { Update: infer U } ? U : Partial<ResolveRow<T>>;

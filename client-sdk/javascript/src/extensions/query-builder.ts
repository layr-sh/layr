import type { LayrClient } from "../core/client";
import { LayrAPIError } from "../core/transport";
import { useData } from "../services/data";
import type { ColumnPath, NestedKeyOf, PathValue } from "../core/paths";

export interface OrderOptions {
  ascending?: boolean;
  nullsFirst?: boolean;
}

export interface InsertOptions {
  onConflict?: string;
}

export type PrimitiveValue = string | number | boolean;

export class QueryBuilder<
  TRow = Record<string, unknown>,
  TInsert = Partial<TRow>,
  TUpdate = Partial<TRow>,
> implements PromiseLike<TRow[]> {
  protected schema: string;
  protected table: string;
  protected client: LayrClient;
  protected selectCols: string = "*";
  protected filterParams: string[] = [];
  protected orderParams: string[] = [];
  protected limitCount?: number;
  protected offsetCount?: number;
  protected primaryKeyId?: string;

  constructor(client: LayrClient, tableWithSchema: string) {
    this.client = client;
    const parts = tableWithSchema.split(".");
    if (parts.length > 1 && parts[0] && parts[1]) {
      this.schema = parts[0];
      this.table = parts[1];
    } else {
      this.schema = "public";
      this.table = tableWithSchema;
    }
  }

  select<K extends keyof TRow & string>(
    columns: K[],
  ): QueryBuilder<Pick<TRow, K>, TInsert, TUpdate>;
  select(columns?: string): QueryBuilder<TRow, TInsert, TUpdate>;
  select(columns: string | (keyof TRow & string)[] = "*"): any {
    if (Array.isArray(columns)) {
      this.selectCols = columns.join(",");
    } else {
      this.selectCols = columns || "*";
    }
    return this;
  }

  join<
    TRelRow = Record<string, unknown>,
    TAlias extends string = string,
    TType extends "left" | "inner" = "left",
  >(
    relation: string,
    options?: {
      type?: TType;
      columns?: (keyof TRelRow & string)[] | string;
      as?: TAlias;
    },
  ): QueryBuilder<
    TRow & { [K in TAlias]: TType extends "inner" ? TRelRow : TRelRow | null },
    TInsert,
    TUpdate
  > {
    const cols = Array.isArray(options?.columns)
      ? options?.columns.join(",")
      : options?.columns || "*";
    const alias = options?.as ? `${options.as}:` : "";
    const modifier = options?.type === "inner" ? "!inner" : "";
    const relationExpr = `${alias}${relation}${modifier}(${cols})`;

    if (this.selectCols && this.selectCols !== "*") {
      this.selectCols += `,${relationExpr}`;
    } else {
      this.selectCols = `*,${relationExpr}`;
    }

    return this as any;
  }

  eq<P extends ColumnPath<TRow>>(
    column: P,
    value: P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any,
  ): this {
    const colStr = String(column);
    if (colStr === "id" || colStr.endsWith("_id")) {
      this.primaryKeyId = String(value);
    }
    this.filterParams.push(`${encodeURIComponent(colStr)}=eq.${encodeURIComponent(String(value))}`);
    return this;
  }

  neq<P extends ColumnPath<TRow>>(
    column: P,
    value: P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any,
  ): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=neq.${encodeURIComponent(String(value))}`,
    );
    return this;
  }

  gt<P extends ColumnPath<TRow>>(
    column: P,
    value: P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any,
  ): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=gt.${encodeURIComponent(String(value))}`,
    );
    return this;
  }

  gte<P extends ColumnPath<TRow>>(
    column: P,
    value: P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any,
  ): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=gte.${encodeURIComponent(String(value))}`,
    );
    return this;
  }

  lt<P extends ColumnPath<TRow>>(
    column: P,
    value: P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any,
  ): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=lt.${encodeURIComponent(String(value))}`,
    );
    return this;
  }

  lte<P extends ColumnPath<TRow>>(
    column: P,
    value: P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any,
  ): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=lte.${encodeURIComponent(String(value))}`,
    );
    return this;
  }

  like<P extends ColumnPath<TRow>>(column: P, pattern: string): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=like.${encodeURIComponent(pattern)}`,
    );
    return this;
  }

  ilike<P extends ColumnPath<TRow>>(column: P, pattern: string): this {
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=ilike.${encodeURIComponent(pattern)}`,
    );
    return this;
  }

  in<P extends ColumnPath<TRow>>(
    column: P,
    values: (P extends NestedKeyOf<TRow> ? PathValue<TRow, P> : any)[],
  ): this {
    const list = values.map((v) => encodeURIComponent(String(v))).join(",");
    this.filterParams.push(`${encodeURIComponent(String(column))}=in.(${list})`);
    return this;
  }

  is<P extends ColumnPath<TRow>>(column: P, value: "null" | "not.null" | null): this {
    const target = value === null ? "null" : value;
    this.filterParams.push(
      `${encodeURIComponent(String(column))}=is.${encodeURIComponent(target)}`,
    );
    return this;
  }

  order<P extends ColumnPath<TRow>>(column: P, options: OrderOptions = {}): this {
    let expr = String(column);
    expr += options.ascending === false ? ".desc" : ".asc";
    if (
      options.nullsFirst === false ||
      (options.nullsFirst === undefined && options.ascending === true)
    ) {
      expr += ".nullslast";
    } else if (options.nullsFirst === true) {
      expr += ".nullsfirst";
    }
    this.orderParams.push(expr);
    return this;
  }

  limit(count: number): this {
    this.limitCount = count;
    return this;
  }

  offset(count: number): this {
    this.offsetCount = count;
    return this;
  }

  range(from: number, to: number): this {
    this.offsetCount = from;
    this.limitCount = to - from + 1;
    return this;
  }

  single(): QueryBuilderSingle<TRow> {
    return new QueryBuilderSingle<TRow>(this as unknown as QueryBuilder<TRow>);
  }

  maybeSingle(): QueryBuilderMaybeSingle<TRow> {
    return new QueryBuilderMaybeSingle<TRow>(this as unknown as QueryBuilder<TRow>);
  }

  buildQueryURL(): string {
    const queryParts: string[] = [];
    if (this.selectCols) {
      queryParts.push(`select=${encodeURIComponent(this.selectCols)}`);
    }
    for (const filter of this.filterParams) {
      queryParts.push(filter);
    }
    if (this.orderParams.length > 0) {
      queryParts.push(`order=${encodeURIComponent(this.orderParams.join(","))}`);
    }
    if (this.limitCount !== undefined) {
      queryParts.push(`limit=${this.limitCount}`);
    }
    if (this.offsetCount !== undefined) {
      queryParts.push(`offset=${this.offsetCount}`);
    }

    const query = queryParts.length > 0 ? `?${queryParts.join("&")}` : "";
    return `${this.client.endpoint}/v1/data/${encodeURIComponent(this.schema)}/${encodeURIComponent(this.table)}${query}`;
  }

  async execute(): Promise<TRow[]> {
    const dataService = useData(this.client);
    const queryParameters: Record<string, string | number | boolean> = {};

    if (this.selectCols) {
      queryParameters["select"] = this.selectCols;
    }
    for (const filter of this.filterParams) {
      const eqIndex = filter.indexOf("=");
      if (eqIndex !== -1) {
        const key = decodeURIComponent(filter.slice(0, eqIndex));
        const val = decodeURIComponent(filter.slice(eqIndex + 1));
        queryParameters[key] = val;
      }
    }
    if (this.orderParams.length > 0) {
      queryParameters["order"] = this.orderParams.join(",");
    }
    if (this.limitCount !== undefined) {
      queryParameters["limit"] = this.limitCount;
    }
    if (this.offsetCount !== undefined) {
      queryParameters["offset"] = this.offsetCount;
    }

    const response = await dataService.records.list<TRow>(this.schema, this.table, queryParameters);
    return response;
  }

  insert(record: TInsert, options?: InsertOptions): Promise<TRow>;
  insert(records: TInsert[], options?: InsertOptions): Promise<TRow[]>;
  async insert(records: TInsert | TInsert[], options?: InsertOptions): Promise<TRow | TRow[]> {
    const data = useData(this.client);
    const response = await data.records.create<TRow | TRow[]>(
      this.schema,
      this.table,
      records,
      options?.onConflict,
    );
    if (!Array.isArray(records) && !Array.isArray(response)) {
      return response as TRow;
    }
    return (Array.isArray(response) ? response : [response]) as TRow[];
  }

  upsert(record: TInsert, options?: InsertOptions): Promise<TRow>;
  upsert(records: TInsert[], options?: InsertOptions): Promise<TRow[]>;
  async upsert(records: TInsert | TInsert[], options?: InsertOptions): Promise<TRow | TRow[]> {
    return this.insert(records as any, options);
  }

  async update(values: TUpdate, recordId?: string): Promise<TRow> {
    const targetId = recordId ?? this.primaryKeyId;
    if (!targetId) {
      throw new Error(
        'update requires a record ID (either specified via eq("id", ...) or passed to update())',
      );
    }
    const data = useData(this.client);
    return await data.records.update<TRow>(this.schema, this.table, targetId, values);
  }

  async delete(recordId?: string): Promise<void> {
    const targetId = recordId ?? this.primaryKeyId;
    if (!targetId) {
      throw new Error(
        'delete requires a record ID (either specified via eq("id", ...) or passed to delete())',
      );
    }
    const data = useData(this.client);
    await data.records.delete(this.schema, this.table, targetId);
  }

  // oxlint-disable-next-line unicorn/no-thenable -- PromiseLike support makes query builders awaitable.
  then<TResult1 = TRow[], TResult2 = never>(
    onfulfilled?: ((value: TRow[]) => TResult1 | PromiseLike<TResult1>) | null,
    onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null,
  ): Promise<TResult1 | TResult2> {
    return this.execute().then(onfulfilled, onrejected);
  }

  catch<TResult = never>(
    onrejected?: ((reason: unknown) => TResult | PromiseLike<TResult>) | null,
  ): Promise<TRow[] | TResult> {
    return this.execute().catch(onrejected);
  }

  finally(onfinally?: (() => void) | null): Promise<TRow[]> {
    return this.execute().finally(onfinally);
  }
}

export class QueryBuilderSingle<T> implements PromiseLike<T> {
  constructor(private parent: QueryBuilder<T>) {}

  async execute(): Promise<T> {
    const results = await this.parent.execute();
    if (results.length !== 1) {
      throw new LayrAPIError({
        status: 404,
        detail: `Expected single record, found ${results.length}`,
      });
    }
    return results[0];
  }

  // oxlint-disable-next-line unicorn/no-thenable -- PromiseLike support makes single-row queries awaitable.
  then<TResult1 = T, TResult2 = never>(
    onfulfilled?: ((value: T) => TResult1 | PromiseLike<TResult1>) | null,
    onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null,
  ): Promise<TResult1 | TResult2> {
    return this.execute().then(onfulfilled, onrejected);
  }

  catch<TResult = never>(
    onrejected?: ((reason: unknown) => TResult | PromiseLike<TResult>) | null,
  ): Promise<T | TResult> {
    return this.execute().catch(onrejected);
  }

  finally(onfinally?: (() => void) | null): Promise<T> {
    return this.execute().finally(onfinally);
  }
}

export class QueryBuilderMaybeSingle<T> implements PromiseLike<T | null> {
  constructor(private parent: QueryBuilder<T>) {}

  async execute(): Promise<T | null> {
    const results = await this.parent.execute();
    if (results.length === 0) return null;
    if (results.length === 1) return results[0];
    throw new LayrAPIError({
      status: 400,
      detail: `Expected at most one record, found ${results.length}`,
    });
  }

  // oxlint-disable-next-line unicorn/no-thenable -- PromiseLike support makes optional single-row queries awaitable.
  then<TResult1 = T | null, TResult2 = never>(
    onfulfilled?: ((value: T | null) => TResult1 | PromiseLike<TResult1>) | null,
    onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null,
  ): Promise<TResult1 | TResult2> {
    return this.execute().then(onfulfilled, onrejected);
  }

  catch<TResult = never>(
    onrejected?: ((reason: unknown) => TResult | PromiseLike<TResult>) | null,
  ): Promise<(T | null) | TResult> {
    return this.execute().catch(onrejected);
  }

  finally(onfinally?: (() => void) | null): Promise<T | null> {
    return this.execute().finally(onfinally);
  }
}

import { describe, it, expectTypeOf } from 'vitest';
import type {
  GenericDatabase,
  GenericTable,
  ResolveTable,
  ResolveRow,
  ResolveInsert,
  ResolveUpdate,
} from '../../src/core/database';

interface TestDatabase {
  public: {
    Tables: {
      todos: {
        Row: { id: number; title: string; completed: boolean };
        Insert: { id?: number; title: string };
        Update: { title?: string; completed?: boolean };
      };
      categories: {
        Row: { id: string; name: string; metadata: { icon: string; count: number } };
      };
      insert_only_table: {
        Row: { id: string; content: string };
        Insert: { content: string };
      };
      update_only_table: {
        Row: { id: string; score: number };
        Update: { score?: number };
      };
    };
  };
  auth: {
    Tables: {
      users: {
        Row: {
          id: string;
          email: string;
          role: 'admin' | 'user' | 'guest';
          created_at: string;
          last_signed_in: string | null;
        };
        Insert: {
          id?: string;
          email: string;
          role?: 'admin' | 'user' | 'guest';
        };
        Update: {
          email?: string;
          role?: 'admin' | 'user' | 'guest';
          last_signed_in?: string | null;
        };
      };
    };
  };
}

interface MultiTenantDatabase {
  tenant_alpha: {
    Tables: {
      customers: {
        Row: { id: string; alphaTier: 'bronze' | 'silver' | 'gold' };
      };
    };
  };
  tenant_beta: {
    Tables: {
      customers: {
        Row: { id: string; betaPoints: number };
      };
    };
  };
  tenant_gamma: {
    Tables: {
      customers: {
        Row: { id: string; gammaCredits: bigint };
      };
    };
  };
}

declare const UserIdBrand: unique symbol;
type BrandedUserId = string & { readonly [UserIdBrand]: true };

interface RichTypesDatabase {
  public: {
    Tables: {
      articles: {
        Row: {
          readonly id: string;
          slug: string;
          tags: readonly string[];
          status: 'draft' | 'published' | 'archived';
          author: {
            name: string;
            social?: {
              twitter?: string;
              github?: string;
            };
          };
          publishedAt: string | null;
        };
      };
      asymmetric_records: {
        Row: {
          id: string;
          computedHash: string;
          status: 'active' | 'revoked';
          createdAt: string;
        };
        Insert: {
          rawSecret: string;
          status?: 'active';
        };
        Update: {
          status?: 'revoked';
        };
      };
      discriminated_events: {
        Row: {
          eventId: string;
          payload:
            | { type: 'click'; x: number; y: number }
            | { type: 'keypress'; key: string }
            | { type: 'hover'; targetId: string };
        };
      };
      tree_nodes: {
        Row: {
          nodeId: string;
          parentId: string | null;
          children: {
            nodeId: string;
            name: string;
          }[];
        };
      };
      branded_and_tuples: {
        Row: {
          userId: BrandedUserId;
          geoCoordinates: [longitude: number, latitude: number];
          timestampMs: bigint;
        };
      };
    };
  };
}

describe('Database Contract & Type Resolver', () => {
  describe('Standard Schema Resolution', () => {
    it('resolves table from global Database schema', () => {
      type Resolved = ResolveTable<TestDatabase, 'public', 'todos'>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<{ id: number; title: string; completed: boolean }>();
      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<{ id?: number; title: string }>();
      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<{ title?: string; completed?: boolean }>();
    });

    it('resolves table from non-public schemas', () => {
      type ResolvedUser = ResolveTable<TestDatabase, 'auth', 'users'>;
      expectTypeOf<ResolveRow<ResolvedUser>>().toEqualTypeOf<{
        id: string;
        email: string;
        role: 'admin' | 'user' | 'guest';
        created_at: string;
        last_signed_in: string | null;
      }>();
      expectTypeOf<ResolveInsert<ResolvedUser>>().toEqualTypeOf<{
        id?: string;
        email: string;
        role?: 'admin' | 'user' | 'guest';
      }>();
      expectTypeOf<ResolveUpdate<ResolvedUser>>().toEqualTypeOf<{
        email?: string;
        role?: 'admin' | 'user' | 'guest';
        last_signed_in?: string | null;
      }>();
    });

    it('resolves ad-hoc model when provided on GenericDatabase', () => {
      interface AdHocTodo {
        id: string;
        done: boolean;
      }
      type Resolved = ResolveTable<GenericDatabase, 'public', 'todos', AdHocTodo>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<AdHocTodo>();
      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<Partial<AdHocTodo>>();
      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<Partial<AdHocTodo>>();
    });

    it('resolves ad-hoc model override on typed Database', () => {
      interface ExternalBillingAccount {
        stripeCustomerId: string;
        currency: 'usd' | 'eur';
        balanceCents: number;
      }
      type Resolved = ResolveTable<TestDatabase, 'public', 'external_billing', ExternalBillingAccount>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<ExternalBillingAccount>();
      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<Partial<ExternalBillingAccount>>();
      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<Partial<ExternalBillingAccount>>();
    });

    it('falls back to generic record when untyped', () => {
      type Resolved = ResolveTable<GenericDatabase, 'public', 'untyped_table'>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<Record<string, unknown>>();
    });

    it('infers Partial<Row> when Insert or Update is omitted from table definition', () => {
      type Resolved = ResolveTable<TestDatabase, 'public', 'categories'>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<{
        id: string;
        name: string;
        metadata: { icon: string; count: number };
      }>();
      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<
        Partial<{ id: string; name: string; metadata: { icon: string; count: number } }>
      >();
      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<
        Partial<{ id: string; name: string; metadata: { icon: string; count: number } }>
      >();
    });

    it('infers Partial<Row> for Update when only Insert is provided', () => {
      type Resolved = ResolveTable<TestDatabase, 'public', 'insert_only_table'>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<{ id: string; content: string }>();
      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<{ content: string }>();
      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<Partial<{ id: string; content: string }>>();
    });

    it('infers Partial<Row> for Insert when only Update is provided', () => {
      type Resolved = ResolveTable<TestDatabase, 'public', 'update_only_table'>;
      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<{ id: string; score: number }>();
      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<Partial<{ id: string; score: number }>>();
      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<{ score?: number }>();
    });
  });

  describe('Multi-Schema & Multi-Tenant Isolation', () => {
    it('strictly isolates identically named tables across 3 different schemas', () => {
      type AlphaCustomer = ResolveTable<MultiTenantDatabase, 'tenant_alpha', 'customers'>;
      type BetaCustomer = ResolveTable<MultiTenantDatabase, 'tenant_beta', 'customers'>;
      type GammaCustomer = ResolveTable<MultiTenantDatabase, 'tenant_gamma', 'customers'>;

      expectTypeOf<ResolveRow<AlphaCustomer>>().toEqualTypeOf<{
        id: string;
        alphaTier: 'bronze' | 'silver' | 'gold';
      }>();

      expectTypeOf<ResolveRow<BetaCustomer>>().toEqualTypeOf<{
        id: string;
        betaPoints: number;
      }>();

      expectTypeOf<ResolveRow<GammaCustomer>>().toEqualTypeOf<{
        id: string;
        gammaCredits: bigint;
      }>();

      // Ensure no property bleed between any schema
      expectTypeOf<ResolveRow<AlphaCustomer>>().not.toEqualTypeOf<ResolveRow<BetaCustomer>>();
      expectTypeOf<ResolveRow<AlphaCustomer>>().not.toEqualTypeOf<ResolveRow<GammaCustomer>>();
      expectTypeOf<ResolveRow<BetaCustomer>>().not.toEqualTypeOf<ResolveRow<GammaCustomer>>();
    });
  });

  describe('Complex Types: Unions, Asymmetry, Discriminated Unions, Branded Types', () => {
    it('preserves readonly, nested objects, and optional/nullable types in ResolveRow', () => {
      type ResolvedArticle = ResolveTable<RichTypesDatabase, 'public', 'articles'>;
      type Row = ResolveRow<ResolvedArticle>;

      expectTypeOf<Row>().toEqualTypeOf<{
        readonly id: string;
        slug: string;
        tags: readonly string[];
        status: 'draft' | 'published' | 'archived';
        author: {
          name: string;
          social?: {
            twitter?: string;
            github?: string;
          };
        };
        publishedAt: string | null;
      }>();
    });

    it('supports complete structural asymmetry between Row, Insert, and Update', () => {
      type Resolved = ResolveTable<RichTypesDatabase, 'public', 'asymmetric_records'>;

      expectTypeOf<ResolveRow<Resolved>>().toEqualTypeOf<{
        id: string;
        computedHash: string;
        status: 'active' | 'revoked';
        createdAt: string;
      }>();

      expectTypeOf<ResolveInsert<Resolved>>().toEqualTypeOf<{
        rawSecret: string;
        status?: 'active';
      }>();

      expectTypeOf<ResolveUpdate<Resolved>>().toEqualTypeOf<{
        status?: 'revoked';
      }>();
    });

    it('preserves discriminated union structures in Row', () => {
      type Resolved = ResolveTable<RichTypesDatabase, 'public', 'discriminated_events'>;
      type EventRow = ResolveRow<Resolved>;

      expectTypeOf<EventRow['payload']>().toEqualTypeOf<
        | { type: 'click'; x: number; y: number }
        | { type: 'keypress'; key: string }
        | { type: 'hover'; targetId: string }
      >();
    });

    it('preserves branded types, tuples, and bigints in Row', () => {
      type Resolved = ResolveTable<RichTypesDatabase, 'public', 'branded_and_tuples'>;
      type Row = ResolveRow<Resolved>;

      expectTypeOf<Row['userId']>().toEqualTypeOf<BrandedUserId>();
      expectTypeOf<Row['geoCoordinates']>().toEqualTypeOf<[longitude: number, latitude: number]>();
      expectTypeOf<Row['timestampMs']>().toEqualTypeOf<bigint>();
    });

    it('preserves hierarchical recursive tree structures in Row', () => {
      type Resolved = ResolveTable<RichTypesDatabase, 'public', 'tree_nodes'>;
      type Row = ResolveRow<Resolved>;

      expectTypeOf<Row>().toEqualTypeOf<{
        nodeId: string;
        parentId: string | null;
        children: {
          nodeId: string;
          name: string;
        }[];
      }>();
    });
  });

  describe('Edge Cases & Degraded Inputs', () => {
    it('falls back to GenericTable when table or schema does not exist in schema definition', () => {
      type NonExistentTable = ResolveTable<TestDatabase, 'public', 'unknown_table'>;
      expectTypeOf<ResolveRow<NonExistentTable>>().toEqualTypeOf<Record<string, unknown>>();

      type NonExistentSchema = ResolveTable<TestDatabase, 'unknown_schema', 'todos'>;
      expectTypeOf<ResolveRow<NonExistentSchema>>().toEqualTypeOf<Record<string, unknown>>();
    });

    it('handles empty or non-object database types gracefully', () => {
      type NullDB = ResolveTable<null, 'public', 'todos'>;
      expectTypeOf<ResolveRow<NullDB>>().toEqualTypeOf<Record<string, unknown>>();

      type UndefinedDB = ResolveTable<undefined, 'public', 'todos'>;
      expectTypeOf<ResolveRow<UndefinedDB>>().toEqualTypeOf<Record<string, unknown>>();

      type PrimitiveDB = ResolveTable<string, 'public', 'todos'>;
      expectTypeOf<ResolveRow<PrimitiveDB>>().toEqualTypeOf<Record<string, unknown>>();

      type NumberDB = ResolveTable<number, 'public', 'todos'>;
      expectTypeOf<ResolveRow<NumberDB>>().toEqualTypeOf<Record<string, unknown>>();

      type EmptyObjectDB = ResolveTable<{}, 'public', 'todos'>;
      expectTypeOf<ResolveRow<EmptyObjectDB>>().toEqualTypeOf<Record<string, unknown>>();
    });

    it('resolves direct helper types on arbitrary generic structures', () => {
      expectTypeOf<ResolveRow<GenericTable>>().toEqualTypeOf<Record<string, unknown>>();
      expectTypeOf<ResolveInsert<GenericTable>>().toEqualTypeOf<Record<string, unknown> | undefined>();
      expectTypeOf<ResolveUpdate<GenericTable>>().toEqualTypeOf<Record<string, unknown> | undefined>();

      expectTypeOf<ResolveRow<{ Row: { a: 1 } }>>().toEqualTypeOf<{ a: 1 }>();
      expectTypeOf<ResolveInsert<{ Row: { a: 1 }; Insert: { a?: 1 } }>>().toEqualTypeOf<{ a?: 1 }>();
      expectTypeOf<ResolveUpdate<{ Row: { a: 1 }; Update: { a?: 2 } }>>().toEqualTypeOf<{ a?: 2 }>();
    });
  });
});

import { describe, it, expectTypeOf } from 'vitest';
import { createClient } from '../../src/core/client';
import type { QueryBuilder, QueryBuilderSingle, QueryBuilderMaybeSingle } from '../../src/extensions/query-builder';
import type { Join } from '../../src/core/paths';
import { useData } from '../../src/services/data';

interface SampleDatabase {
  public: {
    Tables: {
      todos: {
        Row: {
          id: number;
          title: string;
          completed: boolean;
          metadata: {
            priority: 'low' | 'high';
            tags: string[];
          };
        };
        Insert: {
          title: string;
          completed?: boolean;
          metadata?: {
            priority: 'low' | 'high';
            tags: string[];
          };
        };
        Update: {
          title?: string;
          completed?: boolean;
        };
      };
      users: {
        Row: {
          id: string;
          name: string;
          email: string;
        };
        Insert: {
          id?: string;
          name: string;
          email: string;
        };
        Update: {
          name?: string;
          email?: string;
        };
      };
      categories: {
        Row: {
          id: string;
          name: string;
          parentId: string | null;
        };
        Insert: {
          id?: string;
          name: string;
          parentId?: string | null;
        };
        Update: {
          name?: string;
          parentId?: string | null;
        };
      };
      payments: {
        Row: {
          id: string;
          amountCents: number;
          paymentMethod:
            | { type: 'credit_card'; brand: 'visa' | 'mastercard'; last4: string }
            | { type: 'bank_transfer'; iban: string };
        };
        Insert: {
          amountCents: number;
          paymentMethod:
            | { type: 'credit_card'; brand: 'visa' | 'mastercard'; last4: string }
            | { type: 'bank_transfer'; iban: string };
        };
        Update: {
          amountCents?: number;
        };
      };
      user_profiles: {
        Row: {
          userId: string;
          profile: {
            displayName: string;
            location: {
              city: string;
              geo: {
                coordinates: {
                  latitude: number;
                  longitude: number;
                };
              };
            };
            settings: {
              notifications: {
                channels: {
                  email: boolean;
                  sms: boolean;
                };
              };
              theme: 'light' | 'dark' | 'system';
            };
            tags: string[];
          };
        };
        Insert: {
          userId: string;
          profile: {
            displayName: string;
            location: {
              city: string;
              geo: {
                coordinates: {
                  latitude: number;
                  longitude: number;
                };
              };
            };
            settings: {
              notifications: {
                channels: {
                  email: boolean;
                  sms: boolean;
                };
              };
              theme: 'light' | 'dark' | 'system';
            };
            tags: string[];
          };
        };
        Update: {
          profile?: Partial<{
            displayName: string;
          }>;
        };
      };
    };
  };
  audit: {
    Tables: {
      logs: {
        Row: {
          logId: string;
          message: string;
          timestamp: number;
        };
        Insert: {
          message: string;
        };
        Update: {
          message?: string;
        };
      };
    };
  };
  billing: {
    Tables: {
      invoices: {
        Row: {
          invoiceId: string;
          amountCents: number;
          status: 'draft' | 'paid' | 'void';
        };
        Insert: {
          amountCents: number;
          status?: 'draft' | 'paid' | 'void';
        };
        Update: {
          amountCents?: number;
          status?: 'draft' | 'paid' | 'void';
        };
      };
    };
  };
  enterprise: {
    Tables: {
      orgs: {
        Row: {
          orgId: string;
          corp: {
            division: {
              team: {
                lead: {
                  email: string;
                  yearsOfExperience: number;
                  active: boolean;
                };
              };
            };
          };
        };
        Insert: {
          corp: {
            division: {
              team: {
                lead: {
                  email: string;
                  yearsOfExperience: number;
                  active: boolean;
                };
              };
            };
          };
        };
        Update: {
          corp?: any;
        };
      };
    };
  };
  e_commerce: {
    Tables: {
      customers: {
        Row: {
          id: string;
          email: string;
          tier: 'standard' | 'vip';
          address: {
            city: string;
            country: string;
          };
        };
        Insert: {
          email: string;
          tier?: 'standard' | 'vip';
          address: {
            city: string;
            country: string;
          };
        };
        Update: {
          email?: string;
          tier?: 'standard' | 'vip';
          address?: Partial<{
            city: string;
            country: string;
          }>;
        };
      };
      orders: {
        Row: {
          id: string;
          customerId: string;
          totalAmount: number;
          status: 'pending' | 'processing' | 'shipped' | 'delivered' | 'cancelled';
          trackingNumber: string | null;
          discountCode: string | null;
          createdAt: string;
        };
        Insert: {
          customerId: string;
          totalAmount: number;
          status?: 'pending' | 'processing' | 'shipped' | 'delivered' | 'cancelled';
          discountCode?: string | null;
        };
        Update: {
          status?: 'pending' | 'processing' | 'shipped' | 'delivered' | 'cancelled';
          trackingNumber?: string | null;
          discountCode?: string | null;
        };
      };
      order_items: {
        Row: {
          itemId: string;
          orderId: string;
          sku: string;
          quantity: number;
          unitPrice: number;
        };
        Insert: {
          orderId: string;
          sku: string;
          quantity: number;
          unitPrice: number;
        };
        Update: {
          quantity?: number;
          unitPrice?: number;
        };
      };
      reviews: {
        Row: {
          reviewId: string;
          orderId: string;
          rating: 1 | 2 | 3 | 4 | 5;
          comment: string | null;
          upvotes: number;
        };
        Insert: {
          orderId: string;
          rating: 1 | 2 | 3 | 4 | 5;
          comment?: string | null;
        };
        Update: {
          rating?: 1 | 2 | 3 | 4 | 5;
          comment?: string | null;
          upvotes?: number;
        };
      };
    };
  };
}

describe('Vitest Type Testing: Comprehensive Schema & QueryBuilder Edge Cases', () => {
  const client = createClient<SampleDatabase>({ endpoint: 'https://api.layr.sh' });
  const untypedClient = createClient({ endpoint: 'https://api.layr.sh' });
  const data = useData(client);
  const untypedData = useData(untypedClient);

  describe('1. Schema Resolution & Table Validation', () => {
    it('resolves known tables with exact Row, Insert, and Update types', () => {
      const qb = data.from('todos');

      expectTypeOf(qb).toEqualTypeOf<
        QueryBuilder<
          SampleDatabase['public']['Tables']['todos']['Row'],
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('resolves explicit public schema identically to default .from()', () => {
      const qbDefault = data.from('todos');
      const qbExplicit = data.schema('public').from('todos');

      expectTypeOf(qbExplicit).toEqualTypeOf(qbDefault);
    });

    it('rejects nonexistent table names in typed client', () => {
      // @ts-expect-error - 'non_existent_table' is not in SampleDatabase['public']['Tables']
      () => data.from('non_existent_table');

      // @ts-expect-error - 'users' is not in audit schema
      () => data.schema('audit').from('users');
    });

    it('rejects nonexistent schemas in typed client', () => {
      // @ts-expect-error - 'nonexistent_schema' does not exist on SampleDatabase
      () => data.schema('nonexistent_schema');
    });

    it('rejects tables belonging to other schemas without switching schema', () => {
      // @ts-expect-error - 'invoices' belongs to billing schema, not public
      () => data.from('invoices');

      // @ts-expect-error - 'orders' belongs to e_commerce schema, not public
      () => data.from('orders');
    });
  });

  describe('2. Column Selection & Narrowing Edge Cases', () => {
    it('preserves full Row type when selecting with wildcard or default', () => {
      const wildcard = data.from('todos').select('*');
      expectTypeOf(wildcard).toEqualTypeOf<
        QueryBuilder<
          SampleDatabase['public']['Tables']['todos']['Row'],
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();

      const defaultSelect = data.from('todos').select();
      expectTypeOf(defaultSelect).toEqualTypeOf<
        QueryBuilder<
          SampleDatabase['public']['Tables']['todos']['Row'],
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('narrows return type to subset of selected columns', () => {
      const narrowed = data.from('todos').select(['id', 'title']);

      expectTypeOf(narrowed).toEqualTypeOf<
        QueryBuilder<
          { id: number; title: string },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('narrows return type to a single selected column', () => {
      const singleCol = data.from('todos').select(['id']);

      expectTypeOf(singleCol).toEqualTypeOf<
        QueryBuilder<
          { id: number },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('narrows return type to three selected columns', () => {
      const threeCols = data.from('todos').select(['id', 'title', 'completed']);

      expectTypeOf(threeCols).toEqualTypeOf<
        QueryBuilder<
          { id: number; title: string; completed: boolean },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('narrows return type with nested object column', () => {
      const withObj = data.from('todos').select(['id', 'metadata']);

      expectTypeOf(withObj).toEqualTypeOf<
        QueryBuilder<
          {
            id: number;
            metadata: {
              priority: 'low' | 'high';
              tags: string[];
            };
          },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('supports chained select calls narrowing down further', () => {
      const twoCols = data.from('todos').select(['id', 'title']);
      const oneCol = twoCols.select(['id']);

      expectTypeOf(oneCol).toEqualTypeOf<
        QueryBuilder<
          { id: number },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('rejects invalid column names in select array', () => {
      // @ts-expect-error - 'invalid_column' is not a key of todos Row
      () => data.from('todos').select(['id', 'invalid_column']);
    });

    it('preserves builder chaining on narrowed projections and prunes unselected columns', () => {
      const narrowed = data.from('todos').select(['id', 'title']);
      const chained = narrowed.eq('title', 'Test');

      expectTypeOf(chained).toEqualTypeOf<
        QueryBuilder<
          { id: number; title: string },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();

      // @ts-expect-error - 'completed' was pruned from projection
      () => narrowed.eq('completed', true);

      // @ts-expect-error - 'completed' cannot be used in order after being pruned
      () => narrowed.order('completed');
    });
  });

  describe('3. Predicates & Filtering Type Constraints', () => {
    const qb = data.from('todos');

    it('enforces value type match for equality and comparison operators', () => {
      qb.eq('id', 1);
      qb.neq('id', 2);
      qb.gt('id', 0);
      qb.gte('id', 1);
      qb.lt('id', 100);
      qb.lte('id', 50);

      // @ts-expect-error - id is number, not string
      () => qb.eq('id', '1');

      // @ts-expect-error - id is number, not boolean
      () => qb.neq('id', true);

      // @ts-expect-error - completed is boolean, not number
      () => qb.gt('completed', 0);

      // @ts-expect-error - completed is boolean, not string
      () => qb.lte('completed', 'false');
    });

    it('supports chained range filters on the same column', () => {
      const rangeFiltered = qb.gte('id', 10).lte('id', 50);
      expectTypeOf(rangeFiltered).toEqualTypeOf(qb);
    });

    it('supports chained inequality filters on the same column', () => {
      const multiExclusion = qb.neq('title', 'Draft').neq('title', 'Archived');
      expectTypeOf(multiExclusion).toEqualTypeOf(qb);
    });

    it('enforces array elements type in .in() operator', () => {
      qb.in('metadata.priority', ['low', 'high']);
      qb.in('id', [1, 2, 3]);
      qb.in('title', ['Task A', 'Task B']);

      // @ts-expect-error - invalid priority enum value
      () => qb.in('metadata.priority', ['critical']);

      // @ts-expect-error - string array passed to number column
      () => qb.in('id', ['1', '2']);

      // @ts-expect-error - number array passed to string column
      () => qb.in('title', [10, 20]);
    });

    it('enforces valid options for .is() null checks', () => {
      qb.is('completed', 'null');
      qb.is('completed', 'not.null');
      qb.is('completed', null);

      // @ts-expect-error - 'undefined' is not a valid IS operator target
      () => qb.is('completed', 'undefined');

      // @ts-expect-error - boolean literal false is not a valid IS target
      () => qb.is('completed', false);

      // @ts-expect-error - arbitrary string is not a valid IS target
      () => qb.is('completed', 'some_string');

      // @ts-expect-error - number is not a valid IS target
      () => qb.is('completed', 0);
    });

    it('enforces string patterns for like and ilike', () => {
      qb.like('title', '%task%');
      qb.ilike('title', 'TASK%');

      // @ts-expect-error - pattern must be string
      () => qb.like('title', 12345);

      // @ts-expect-error - pattern must be string, not boolean
      () => qb.ilike('title', true);
    });

    it('validates column names for .order() sorting', () => {
      qb.order('id', { ascending: true, nullsFirst: false });
      qb.order('id', { ascending: false });
      qb.order('metadata.priority', { ascending: false });

      // @ts-expect-error - invalid column name for ordering
      () => qb.order('nonexistent_column');

      // @ts-expect-error - invalid nested path for ordering
      () => qb.order('metadata.nonexistent');
    });

    it('maintains QueryBuilder type on pagination modifiers', () => {
      const paginated = qb.limit(10).offset(20).range(0, 9);
      expectTypeOf(paginated).toEqualTypeOf(qb);
    });
  });

  describe('4. Deep Nested JSON Paths (Up to 5 Levels Deep & JSON Arrows)', () => {
    const upQb = data.from('user_profiles');

    it('validates 4-level deep nested dot paths and value types', () => {
      upQb.eq('profile.location.city', 'Jakarta');
      upQb.eq('profile.location.geo.coordinates.latitude', -6.2088);
      upQb.eq('profile.location.geo.coordinates.longitude', 106.8456);
      upQb.eq('profile.settings.notifications.channels.email', true);
      upQb.eq('profile.settings.theme', 'system');

      // @ts-expect-error - latitude must be number, not string
      () => upQb.eq('profile.location.geo.coordinates.latitude', '-6.2088');

      // @ts-expect-error - email notification channel must be boolean
      () => upQb.eq('profile.settings.notifications.channels.email', 'yes');

      // @ts-expect-error - 'blue' is not assignable to 'light' | 'dark' | 'system'
      () => upQb.eq('profile.settings.theme', 'blue');

      // @ts-expect-error - nonexistent nested property
      () => upQb.eq('profile.location.invalid_field', 'test');
    });

    it('validates 5-level deep nested dot paths on enterprise schema', () => {
      const entQb = data.schema('enterprise').from('orgs');

      entQb.eq('corp.division.team.lead.email', 'lead@layr.sh');
      entQb.eq('corp.division.team.lead.yearsOfExperience', 8);
      entQb.eq('corp.division.team.lead.active', true);

      // @ts-expect-error - email must be string, not number
      () => entQb.eq('corp.division.team.lead.email', 12345);

      // @ts-expect-error - yearsOfExperience must be number, not string
      () => entQb.eq('corp.division.team.lead.yearsOfExperience', 'eight');

      // @ts-expect-error - nonexistent property at level 5
      () => entQb.eq('corp.division.team.lead.nonexistent', 'test');

      // @ts-expect-error - nonexistent property at level 2
      () => entQb.eq('corp.invalid_division', 'test');
    });

    it('allows PostgreSQL JSON arrow operators on top-level JSONB columns', () => {
      upQb.eq('profile->>displayName', 'Alex');
      upQb.eq('profile->location->geo->coordinates->>latitude', -6.2088);
      upQb.eq('profile->location->geo->coordinates->latitude', -6.2088);
      upQb.eq("profile->'location'->>'city'", 'Jakarta');
    });
  });

  describe('5. Mutation Typing: Single vs Batch Insert, Upsert, Update, Delete', () => {
    const qb = data.from('todos');

    it('overloads insert: single record returns Promise<TRow>, array returns Promise<TRow[]>', () => {
      const single = qb.insert({ title: 'Single' });
      expectTypeOf(single).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row']>>();

      const batch = qb.insert([{ title: 'Batch 1' }, { title: 'Batch 2' }]);
      expectTypeOf(batch).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row'][]>>();

      // @ts-expect-error - 'nonexistent_key' is not in Insert schema
      () => qb.insert({ title: 'Invalid', nonexistent_key: true });

      // @ts-expect-error - title is required in Insert schema
      () => qb.insert({});

      // @ts-expect-error - title must be string, not number
      () => qb.insert({ title: 123 });
    });

    it('supports inserting batches with mixed optional fields', () => {
      const mixedBatch = qb.insert([
        { title: 'Task without optional' },
        { title: 'Task with optional', completed: true, metadata: { priority: 'high', tags: ['work'] } },
      ]);
      expectTypeOf(mixedBatch).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row'][]>>();

      // @ts-expect-error - batch with one invalid item is rejected
      () => qb.insert([{ title: 'Valid' }, { title: 999 }]);
    });

    it('supports InsertOptions on .insert() and .upsert()', () => {
      const singleWithOptions = qb.insert({ title: 'Single' }, { onConflict: 'id' });
      expectTypeOf(singleWithOptions).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row']>>();

      const batchWithOptions = qb.insert([{ title: 'Batch' }], { onConflict: 'id' });
      expectTypeOf(batchWithOptions).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row'][]>>();
    });

    it('overloads upsert: single record returns Promise<TRow>, array returns Promise<TRow[]>', () => {
      const singleUpsert = qb.upsert({ title: 'Upsert Single' }, { onConflict: 'id' });
      expectTypeOf(singleUpsert).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row']>>();

      const batchUpsert = qb.upsert([{ title: 'Upsert 1' }, { title: 'Upsert 2' }]);
      expectTypeOf(batchUpsert).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row'][]>>();

      // @ts-expect-error - title is required in Insert schema
      () => qb.upsert({});
    });

    it('enforces Update contract on .update()', () => {
      const updateResult = qb.update({ completed: true }, '1');
      expectTypeOf(updateResult).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row']>>();

      const emptyUpdate = qb.update({}, '1');
      expectTypeOf(emptyUpdate).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['todos']['Row']>>();

      // @ts-expect-error - id is immutable and not in Update schema
      () => qb.update({ id: 123 }, '1');

      // @ts-expect-error - title must be string
      () => qb.update({ title: 12345 }, '1');

      // @ts-expect-error - unrecognized column
      () => qb.update({ unknown_prop: true }, '1');
    });

    it('returns Promise<void> on .delete()', () => {
      const deleteResult = qb.delete('1');
      expectTypeOf(deleteResult).toEqualTypeOf<Promise<void>>();
    });
  });

  describe('6. Terminal Fetch Operations: single(), maybeSingle(), and PromiseLike contract', () => {
    const qb = data.from('todos');

    it('resolves .single() to Promise<TRow>', () => {
      const single = qb.single();
      expectTypeOf(single).toEqualTypeOf<QueryBuilderSingle<SampleDatabase['public']['Tables']['todos']['Row']>>();
      expectTypeOf(single).toMatchTypeOf<PromiseLike<SampleDatabase['public']['Tables']['todos']['Row']>>();
    });

    it('resolves .maybeSingle() to Promise<TRow | null>', () => {
      const maybeSingle = qb.maybeSingle();
      expectTypeOf(maybeSingle).toEqualTypeOf<
        QueryBuilderMaybeSingle<SampleDatabase['public']['Tables']['todos']['Row']>
      >();
      expectTypeOf(maybeSingle).toMatchTypeOf<
        PromiseLike<SampleDatabase['public']['Tables']['todos']['Row'] | null>
      >();
    });

    it('resolves QueryBuilder instance to Promise<TRow[]>', () => {
      expectTypeOf(qb).toMatchTypeOf<PromiseLike<SampleDatabase['public']['Tables']['todos']['Row'][]>>();
    });

    it('verifies .then(), .catch(), and .finally() method signatures', () => {
      qb.then((rows) => {
        expectTypeOf(rows).toEqualTypeOf<SampleDatabase['public']['Tables']['todos']['Row'][]>();
      });

      qb.single().then((row) => {
        expectTypeOf(row).toEqualTypeOf<SampleDatabase['public']['Tables']['todos']['Row']>();
      });

      qb.maybeSingle().then((row) => {
        expectTypeOf(row).toEqualTypeOf<SampleDatabase['public']['Tables']['todos']['Row'] | null>();
      });
    });

    it('resolves narrowed .single() to exact narrowed Row', () => {
      const narrowedSingle = data.from('todos').select(['id', 'title']).single();
      expectTypeOf(narrowedSingle).toMatchTypeOf<PromiseLike<{ id: number; title: string }>>();
    });

    it('resolves narrowed .maybeSingle() to exact narrowed Row or null', () => {
      const narrowedMaybeSingle = data.from('todos').select(['id']).maybeSingle();
      expectTypeOf(narrowedMaybeSingle).toMatchTypeOf<PromiseLike<{ id: number } | null>>();
    });
  });

  describe('7. Relational Joins & Overloads', () => {
    type UserRow = SampleDatabase['public']['Tables']['users']['Row'];
    type AuditLogRow = SampleDatabase['audit']['Tables']['logs']['Row'];
    type CategoryRow = SampleDatabase['public']['Tables']['categories']['Row'];

    it('supports left join with nullable relation property', () => {
      const leftJoined = data.from('todos').join<UserRow, 'author'>('users', {
        type: 'left',
        as: 'author',
      });

      type ExpectedLeft = SampleDatabase['public']['Tables']['todos']['Row'] & {
        author: UserRow | null;
      };

      expectTypeOf(leftJoined).toEqualTypeOf<
        QueryBuilder<
          ExpectedLeft,
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('supports inner join with non-null relation property', () => {
      const innerJoined = data.from('todos').join<UserRow, 'author', 'inner'>('users', {
        type: 'inner',
        as: 'author',
      });

      type ExpectedInner = SampleDatabase['public']['Tables']['todos']['Row'] & {
        author: UserRow;
      };

      expectTypeOf(innerJoined).toEqualTypeOf<
        QueryBuilder<
          ExpectedInner,
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('supports self-referential left join (e.g. users -> manager)', () => {
      const withManager = data.from('users').join<UserRow, 'manager'>('users', {
        as: 'manager',
      });

      type Expected = UserRow & { manager: UserRow | null };

      expectTypeOf(withManager).toEqualTypeOf<
        QueryBuilder<
          Expected,
          SampleDatabase['public']['Tables']['users']['Insert'],
          SampleDatabase['public']['Tables']['users']['Update']
        >
      >();
    });

    it('supports self-referential 1-to-many relation join (e.g. categories -> subcategories)', () => {
      const withSubcategories = data.from('categories').join<CategoryRow[], 'subcategories'>('categories', {
        as: 'subcategories',
      });

      type Expected = CategoryRow & { subcategories: CategoryRow[] | null };

      expectTypeOf(withSubcategories).toEqualTypeOf<
        QueryBuilder<
          Expected,
          SampleDatabase['public']['Tables']['categories']['Insert'],
          SampleDatabase['public']['Tables']['categories']['Update']
        >
      >();
    });

    it('supports 1-to-many relation join with array child', () => {
      interface CommentRow {
        commentId: string;
        body: string;
      }

      const postWithComments = data.from('todos').join<CommentRow[], 'comments'>('comments', {
        as: 'comments',
      });

      type ExpectedPost = SampleDatabase['public']['Tables']['todos']['Row'] & {
        comments: CommentRow[] | null;
      };

      expectTypeOf(postWithComments).toEqualTypeOf<
        QueryBuilder<
          ExpectedPost,
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('supports chaining multiple relational joins', () => {
      const multiJoined = data
        .from('todos')
        .join<UserRow, 'author'>('users', { as: 'author' })
        .join<AuditLogRow[], 'audit_logs'>('logs', { as: 'audit_logs' });

      type ExpectedMulti = SampleDatabase['public']['Tables']['todos']['Row'] & {
        author: UserRow | null;
        audit_logs: AuditLogRow[] | null;
      };

      expectTypeOf(multiJoined).toEqualTypeOf<
        QueryBuilder<
          ExpectedMulti,
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('allows projection of both parent and joined properties after join', () => {
      const joined = data.from('todos').join<UserRow, 'author', 'inner'>('users', {
        type: 'inner',
        as: 'author',
      });

      const selected = joined.select(['id', 'author']);

      expectTypeOf(selected).toEqualTypeOf<
        QueryBuilder<
          { id: number; author: UserRow },
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('verifies Join<Parent, Alias, Child, IsInner> type utility helper', () => {
      type Parent = { id: string };
      type Child = { name: string };

      type Left = Join<Parent, 'child', Child>;
      expectTypeOf<Left>().toEqualTypeOf<{ id: string; child: Child | null }>();

      type Inner = Join<Parent, 'child', Child, true>;
      expectTypeOf<Inner>().toEqualTypeOf<{ id: string; child: Child }>();

      type ArrayChild = Join<Parent, 'items', Child[], false>;
      expectTypeOf<ArrayChild>().toEqualTypeOf<{ id: string; items: Child[] | null }>();

      type InnerArrayChild = Join<Parent, 'items', Child[], true>;
      expectTypeOf<InnerArrayChild>().toEqualTypeOf<{ id: string; items: Child[] }>();
    });
  });

  describe('8. Ad-Hoc Generic Overrides & Untyped Client', () => {
    interface AdHocArticle {
      articleId: string;
      body: string;
      views: number;
    }

    it('allows ad-hoc generic override on typed client for external tables', () => {
      const adHocQb = data.from<AdHocArticle>('articles');

      expectTypeOf(adHocQb).toEqualTypeOf<
        QueryBuilder<AdHocArticle, Partial<AdHocArticle>, Partial<AdHocArticle>>
      >();

      adHocQb.eq('views', 100);

      // @ts-expect-error - views must be number
      () => adHocQb.eq('views', '100');

      // @ts-expect-error - invalid column on ad-hoc model
      () => adHocQb.eq('invalid_field', 'test');
    });

    it('allows arbitrary strings on untyped client with Record<string, unknown> default', () => {
      const untypedQb = untypedData.from('any_dynamic_table');

      expectTypeOf(untypedQb).toEqualTypeOf<
        QueryBuilder<Record<string, unknown>, Partial<Record<string, unknown>>, Partial<Record<string, unknown>>>
      >();

      untypedQb.eq('dynamic_column', 'any_value');
    });

    it('allows strict ad-hoc typing on untyped client instances', () => {
      const strictAdHoc = untypedData.from<AdHocArticle>('articles');

      expectTypeOf(strictAdHoc).toEqualTypeOf<
        QueryBuilder<AdHocArticle, Partial<AdHocArticle>, Partial<AdHocArticle>>
      >();

      strictAdHoc.eq('articleId', 'art_001');

      // @ts-expect-error - strictly enforces AdHocArticle fields even on untyped client
      () => strictAdHoc.eq('nonexistent', true);
    });

    it('rejects untyped table access on typed client without generic', () => {
      // @ts-expect-error - cannot pass unmapped table without generic override on typed client
      () => data.from('unmapped_table');
    });
  });

  describe('9. Multi-Schema Operations', () => {
    it('supports selecting tables from custom schemas with distinct types', () => {
      const auditQb = data.schema('audit').from('logs');
      expectTypeOf(auditQb).toEqualTypeOf<
        QueryBuilder<
          SampleDatabase['audit']['Tables']['logs']['Row'],
          SampleDatabase['audit']['Tables']['logs']['Insert'],
          SampleDatabase['audit']['Tables']['logs']['Update']
        >
      >();

      const billingQb = data.schema('billing').from('invoices');
      expectTypeOf(billingQb).toEqualTypeOf<
        QueryBuilder<
          SampleDatabase['billing']['Tables']['invoices']['Row'],
          SampleDatabase['billing']['Tables']['invoices']['Insert'],
          SampleDatabase['billing']['Tables']['invoices']['Update']
        >
      >();
    });

    it('rejects cross-schema table queries', () => {
      // @ts-expect-error - 'invoices' is in billing, not audit
      () => data.schema('audit').from('invoices');

      // @ts-expect-error - 'todos' is in public, not billing
      () => data.schema('billing').from('todos');
    });
  });

  describe('10. Nullable, Optional, and Literal Union Edge Cases', () => {
    const ordersQb = data.schema('e_commerce').from('orders');
    const reviewsQb = data.schema('e_commerce').from('reviews');

    it('handles nullable columns with string and null comparisons', () => {
      ordersQb.eq('trackingNumber', 'TRK-123456');
      ordersQb.is('trackingNumber', null);
      ordersQb.is('trackingNumber', 'null');
      ordersQb.is('trackingNumber', 'not.null');

      // @ts-expect-error - number is not assignable to string | null
      () => ordersQb.eq('trackingNumber', 999);
    });

    it('handles string literal union columns strictly', () => {
      ordersQb.eq('status', 'processing');
      ordersQb.eq('status', 'delivered');
      ordersQb.in('status', ['pending', 'shipped', 'delivered']);

      // @ts-expect-error - 'refunded' is not in union 'pending' | 'processing' | 'shipped' | 'delivered' | 'cancelled'
      () => ordersQb.eq('status', 'refunded');

      // @ts-expect-error - 'unknown_status' not allowed in .in() array
      () => ordersQb.in('status', ['pending', 'unknown_status']);
    });

    it('handles numeric literal union columns strictly', () => {
      reviewsQb.eq('rating', 5);
      reviewsQb.eq('rating', 1);

      // @ts-expect-error - 6 is not in 1 | 2 | 3 | 4 | 5
      () => reviewsQb.eq('rating', 6);

      // @ts-expect-error - string '5' is not in 1 | 2 | 3 | 4 | 5
      () => reviewsQb.eq('rating', '5');
    });
  });

  describe('11. Complex Real-World E-Commerce Query Pipeline', () => {
    type CustomerRow = SampleDatabase['e_commerce']['Tables']['customers']['Row'];
    type OrderItemRow = SampleDatabase['e_commerce']['Tables']['order_items']['Row'];
    type ReviewRow = SampleDatabase['e_commerce']['Tables']['reviews']['Row'];

    it('supports 3-way joins across relational entities with nullability preservation', () => {
      const complexPipeline = data
        .schema('e_commerce')
        .from('orders')
        .join<CustomerRow, 'customer', 'inner'>('customers', {
          type: 'inner',
          as: 'customer',
        })
        .join<OrderItemRow[], 'items'>('order_items', {
          as: 'items',
        })
        .join<ReviewRow | null, 'review'>('reviews', {
          as: 'review',
        })
        .eq('status', 'processing')
        .gte('totalAmount', 100)
        .order('createdAt', { ascending: false })
        .limit(10);

      type ExpectedCompositeRow = SampleDatabase['e_commerce']['Tables']['orders']['Row'] & {
        customer: CustomerRow;
        items: OrderItemRow[] | null;
        review: ReviewRow | null;
      };

      expectTypeOf(complexPipeline).toMatchTypeOf<PromiseLike<ExpectedCompositeRow[]>>();

      const maybeSingle = complexPipeline.maybeSingle();
      expectTypeOf(maybeSingle).toMatchTypeOf<PromiseLike<ExpectedCompositeRow | null>>();
    });
  });

  describe('12. Discriminated Union Row Structures in QueryBuilder', () => {
    const payQb = data.from('payments');

    it('preserves discriminated union row in QueryBuilder and types mutations', () => {
      expectTypeOf(payQb).toMatchTypeOf<
        QueryBuilder<
          SampleDatabase['public']['Tables']['payments']['Row'],
          SampleDatabase['public']['Tables']['payments']['Insert'],
          SampleDatabase['public']['Tables']['payments']['Update']
        >
      >();

      payQb.eq('amountCents', 2500);

      // @ts-expect-error - amountCents must be number
      () => payQb.eq('amountCents', '2500');

      const singleInsert = payQb.insert({
        amountCents: 5000,
        paymentMethod: { type: 'credit_card', brand: 'visa', last4: '4242' },
      });
      expectTypeOf(singleInsert).toEqualTypeOf<Promise<SampleDatabase['public']['Tables']['payments']['Row']>>();

      // @ts-expect-error - invalid paymentMethod union type
      () => payQb.insert({
        amountCents: 5000,
        paymentMethod: { type: 'invalid_type' },
      });
    });
  });

  describe('13. Service Wrapper Integration (useData & records)', () => {
    const dataService = useData(client);

    it('infers typed QueryBuilder from useData(client).from()', () => {
      const qb = dataService.from('todos');

      expectTypeOf(qb).toEqualTypeOf<
        QueryBuilder<
          SampleDatabase['public']['Tables']['todos']['Row'],
          SampleDatabase['public']['Tables']['todos']['Insert'],
          SampleDatabase['public']['Tables']['todos']['Update']
        >
      >();
    });

    it('types useData(client).records methods with model generics', async () => {
      type TodoRow = SampleDatabase['public']['Tables']['todos']['Row'];

      const listResult = dataService.records.list<TodoRow>('public', 'todos', {
        limit: 10,
        title: 'Groceries',
      });
      expectTypeOf(listResult).toEqualTypeOf<Promise<TodoRow[]>>();

      const getResult = dataService.records.get<TodoRow>('public', 'todos', '1');
      expectTypeOf(getResult).toEqualTypeOf<Promise<TodoRow>>();

      const createResult = dataService.records.create<TodoRow>('public', 'todos', { title: 'New' });
      expectTypeOf(createResult).toEqualTypeOf<Promise<TodoRow>>();

      const updateResult = dataService.records.update<TodoRow>('public', 'todos', '1', { completed: true });
      expectTypeOf(updateResult).toEqualTypeOf<Promise<TodoRow>>();

      const putResult = dataService.records.put<TodoRow>('public', 'todos', '1', { title: 'Replaced' });
      expectTypeOf(putResult).toEqualTypeOf<Promise<TodoRow>>();

      const deleteResult = dataService.records.delete('public', 'todos', '1');
      expectTypeOf(deleteResult).toEqualTypeOf<Promise<void>>();
    });
  });
});

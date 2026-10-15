import { describe, it, expectTypeOf } from 'vitest';
import type { NestedKeyOf, PathValue, ColumnPath, Join } from '../../src/core/paths';

interface ComplexHierarchy {
  id: string;
  name: string;
  active: boolean;
  score: number;
  tags: string[];
  matrix: number[][];
  nullableField: string | null;
  optionalField?: number;
  metadata: {
    version: number;
    author: {
      firstName: string;
      lastName: string;
      contact: {
        email: string;
        phone: {
          countryCode: string;
          nationalNumber: string;
        };
      };
    };
  };
  items: {
    sku: string;
    price: number;
    details: {
      color: string;
      weightKg: number;
    };
  }[];
}

describe('Paths and Join Type Engine Edge Cases', () => {
  describe('1. NestedKeyOf Recursive Type Extraction', () => {
    it('extracts top-level keys', () => {
      type Keys = NestedKeyOf<ComplexHierarchy>;
      expectTypeOf<'id'>().toMatchTypeOf<Keys>();
      expectTypeOf<'name'>().toMatchTypeOf<Keys>();
      expectTypeOf<'active'>().toMatchTypeOf<Keys>();
      expectTypeOf<'score'>().toMatchTypeOf<Keys>();
      expectTypeOf<'tags'>().toMatchTypeOf<Keys>();
    });

    it('extracts nested keys across multiple object levels', () => {
      type Keys = NestedKeyOf<ComplexHierarchy>;
      expectTypeOf<'metadata.version'>().toMatchTypeOf<Keys>();
      expectTypeOf<'metadata.author.firstName'>().toMatchTypeOf<Keys>();
      expectTypeOf<'metadata.author.contact.email'>().toMatchTypeOf<Keys>();
      expectTypeOf<'metadata.author.contact.phone.countryCode'>().toMatchTypeOf<Keys>();
      expectTypeOf<'metadata.author.contact.phone.nationalNumber'>().toMatchTypeOf<Keys>();
    });

    it('extracts nested properties across arrays of objects', () => {
      type Keys = NestedKeyOf<ComplexHierarchy>;
      expectTypeOf<'items.sku'>().toMatchTypeOf<Keys>();
      expectTypeOf<'items.price'>().toMatchTypeOf<Keys>();
      expectTypeOf<'items.details.color'>().toMatchTypeOf<Keys>();
      expectTypeOf<'items.details.weightKg'>().toMatchTypeOf<Keys>();
    });

    it('does not recurse on primitive arrays', () => {
      type Keys = NestedKeyOf<ComplexHierarchy>;
      expectTypeOf<'tags'>().toMatchTypeOf<Keys>();
      expectTypeOf<'matrix'>().toMatchTypeOf<Keys>();
      // @ts-expect-error - primitive arrays do not have nested property keys
      () => { const _k: Keys = 'tags.length'; };
    });

    it('returns never for primitive types', () => {
      expectTypeOf<NestedKeyOf<string>>().toEqualTypeOf<never>();
      expectTypeOf<NestedKeyOf<number>>().toEqualTypeOf<never>();
      expectTypeOf<NestedKeyOf<boolean>>().toEqualTypeOf<never>();
      expectTypeOf<NestedKeyOf<null>>().toEqualTypeOf<never>();
      expectTypeOf<NestedKeyOf<undefined>>().toEqualTypeOf<never>();
    });
  });

  describe('2. PathValue Type Resolution', () => {
    it('resolves exact types for direct top-level keys', () => {
      expectTypeOf<PathValue<ComplexHierarchy, 'id'>>().toEqualTypeOf<string>();
      expectTypeOf<PathValue<ComplexHierarchy, 'active'>>().toEqualTypeOf<boolean>();
      expectTypeOf<PathValue<ComplexHierarchy, 'score'>>().toEqualTypeOf<number>();
      expectTypeOf<PathValue<ComplexHierarchy, 'tags'>>().toEqualTypeOf<string[]>();
      expectTypeOf<PathValue<ComplexHierarchy, 'nullableField'>>().toEqualTypeOf<string | null>();
      expectTypeOf<PathValue<ComplexHierarchy, 'optionalField'>>().toEqualTypeOf<number | undefined>();
    });

    it('resolves exact types for deep multi-level dot paths', () => {
      expectTypeOf<PathValue<ComplexHierarchy, 'metadata.version'>>().toEqualTypeOf<number>();
      expectTypeOf<PathValue<ComplexHierarchy, 'metadata.author.firstName'>>().toEqualTypeOf<string>();
      expectTypeOf<PathValue<ComplexHierarchy, 'metadata.author.contact.email'>>().toEqualTypeOf<string>();
      expectTypeOf<PathValue<ComplexHierarchy, 'metadata.author.contact.phone.countryCode'>>().toEqualTypeOf<string>();
    });

    it('resolves unknown for invalid or nonexistent paths', () => {
      expectTypeOf<PathValue<ComplexHierarchy, 'nonexistent'>>().toEqualTypeOf<unknown>();
      expectTypeOf<PathValue<ComplexHierarchy, 'metadata.nonexistent'>>().toEqualTypeOf<unknown>();
      expectTypeOf<PathValue<ComplexHierarchy, 'metadata.author.invalid_field'>>().toEqualTypeOf<unknown>();
    });
  });

  describe('3. ColumnPath Pattern Matching', () => {
    type Col = ColumnPath<ComplexHierarchy>;

    it('matches top-level and nested column paths', () => {
      expectTypeOf<'id'>().toMatchTypeOf<Col>();
      expectTypeOf<'metadata.version'>().toMatchTypeOf<Col>();
      expectTypeOf<'metadata.author.contact.email'>().toMatchTypeOf<Col>();
    });

    it('matches PostgreSQL JSON single and double arrow operators', () => {
      expectTypeOf<'metadata->version'>().toMatchTypeOf<Col>();
      expectTypeOf<'metadata->>version'>().toMatchTypeOf<Col>();
      expectTypeOf<'metadata->author->contact->>email'>().toMatchTypeOf<Col>();
      expectTypeOf<"metadata->'author'->>'firstName'">().toMatchTypeOf<Col>();
    });

    it('rejects columns that do not start with a valid top-level property', () => {
      // @ts-expect-error - invalid property prefix
      () => { const _c: Col = 'invalid_prop'; };

      // @ts-expect-error - invalid property prefix for JSON arrow
      () => { const _c: Col = 'invalid_prop->field'; };
    });
  });

  describe('4. Join Type Combinator Edge Cases', () => {
    interface Parent {
      parentId: string;
      title: string;
    }
    interface ChildA {
      childId: string;
      data: string;
    }
    interface ChildB {
      subId: string;
      value: number;
    }

    it('creates left join with nullable child property', () => {
      type Joined = Join<Parent, 'childA', ChildA>;
      expectTypeOf<Joined>().toEqualTypeOf<Parent & { childA: ChildA | null }>();
    });

    it('creates inner join with non-null child property', () => {
      type Joined = Join<Parent, 'childA', ChildA, true>;
      expectTypeOf<Joined>().toEqualTypeOf<Parent & { childA: ChildA }>();
    });

    it('creates 1-to-many left join with array child property', () => {
      type Joined = Join<Parent, 'children', ChildA[], false>;
      expectTypeOf<Joined>().toEqualTypeOf<Parent & { children: ChildA[] | null }>();
    });

    it('creates 1-to-many inner join with non-null array child property', () => {
      type Joined = Join<Parent, 'children', ChildA[], true>;
      expectTypeOf<Joined>().toEqualTypeOf<Parent & { children: ChildA[] }>();
    });

    it('chains multiple relational joins sequentially', () => {
      type MultiJoined = Join<Join<Parent, 'childA', ChildA, true>, 'childB', ChildB, false>;
      expectTypeOf<MultiJoined>().toEqualTypeOf<
        Parent & { childA: ChildA } & { childB: ChildB | null }
      >();
    });
  });
});

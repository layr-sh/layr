import { describe, it, expectTypeOf } from 'vitest';
import type { NestedKeyOf, PathValue, ColumnPath, Join } from '../../src/core/paths';

interface SampleUser {
  id: number;
  profile: {
    age: number;
    address: {
      city: string;
      zip: string;
    };
  };
}

describe('Paths and Join Type Helpers', () => {
  it('extracts nested keys', () => {
    type Keys = NestedKeyOf<SampleUser>;
    expectTypeOf<'profile.age'>().toMatchTypeOf<Keys>();
    expectTypeOf<'profile.address.city'>().toMatchTypeOf<Keys>();
  });

  it('extracts value types at nested path', () => {
    type Age = PathValue<SampleUser, 'profile.age'>;
    type City = PathValue<SampleUser, 'profile.address.city'>;
    expectTypeOf<Age>().toEqualTypeOf<number>();
    expectTypeOf<City>().toEqualTypeOf<string>();
  });

  it('allows JSON arrow operators in ColumnPath', () => {
    type Path = ColumnPath<SampleUser>;
    expectTypeOf<'profile->>age'>().toMatchTypeOf<Path>();
    expectTypeOf<'profile->address->>city'>().toMatchTypeOf<Path>();
  });

  it('joins relations into parent types', () => {
    interface Post {
      id: string;
      title: string;
    }
    type UserWithPosts = Join<SampleUser, 'posts', Post[]>;
    expectTypeOf<UserWithPosts>().toEqualTypeOf<SampleUser & { posts: Post[] | null }>();

    type UserWithInnerPosts = Join<SampleUser, 'posts', Post[], true>;
    expectTypeOf<UserWithInnerPosts>().toEqualTypeOf<SampleUser & { posts: Post[] }>();
  });
});

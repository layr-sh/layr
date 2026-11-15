type Prev = [never, 0, 1, 2, 3, 4, 5];

export type NestedKeyOf<T, Depth extends number = 5> = [Depth] extends [0]
  ? never
  : T extends object
    ? {
        [K in keyof T & string]: T[K] extends (infer E)[]
          ? K | `${K}.${NestedKeyOf<E, Prev[Depth]>}`
          : T[K] extends object
            ? K | `${K}.${NestedKeyOf<T[K], Prev[Depth]>}`
            : K;
      }[keyof T & string]
    : never;

export type PathValue<T, P extends string> = P extends `${infer K}.${infer Rest}`
  ? K extends keyof T
    ? PathValue<T[K], Rest>
    : unknown
  : P extends keyof T
    ? T[P]
    : unknown;

export type ColumnPath<T> =
  | (keyof T & string)
  | NestedKeyOf<T>
  | `${keyof T & string}->${string}`
  | `${keyof T & string}->>${string}`;

export type Join<Parent, Alias extends string, Child, IsInner extends boolean = false> = Parent & {
  [K in Alias]: IsInner extends true ? Child : Child | null;
};

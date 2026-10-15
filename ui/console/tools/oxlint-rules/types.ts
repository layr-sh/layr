import type { RuleTester } from "oxlint/plugins-dev";

/**
 * Oxlint's `dist/plugins-dev.d.ts` declares `CreateRule`, `CreateOnceRule`, and `Context`
 * internally for `RuleTester`, but only exports `RuleTester` and its namespace.
 *
 * Here we extract those exact types so our rules can import them cleanly.
 */
export type Rule = Parameters<RuleTester["run"]>[1];

type CreateRule = Extract<Rule, { create: unknown }>;
export type Context = Parameters<CreateRule["create"]>[0];
export type Variable = ReturnType<Context["sourceCode"]["getDeclaredVariables"]>[number];
export type VariableDeclarator = Parameters<
  NonNullable<ReturnType<CreateRule["create"]>["VariableDeclarator"]>
>[0];

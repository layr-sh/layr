import type { Context, Rule } from "./types.ts";

const defaultTargetFunctions: readonly string[] = [
  "useRuntimeConfig",
  "resolveRuntimeConfig",
] as const;

function unwrapCallExpression(expr: unknown): { calleeName: string } | null {
  let current: unknown = expr;

  while (current && typeof current === "object") {
    if ("type" in current) {
      if (
        current.type === "ParenthesizedExpression" ||
        current.type === "TSNonNullExpression" ||
        current.type === "TSAsExpression" ||
        current.type === "TSTypeAssertion"
      ) {
        if ("expression" in current) {
          current = current.expression;
          continue;
        }
      }
    }
    break;
  }

  if (
    current &&
    typeof current === "object" &&
    "type" in current &&
    current.type === "CallExpression"
  ) {
    if ("callee" in current && current.callee && typeof current.callee === "object") {
      const callee = current.callee;
      if ("type" in callee) {
        if (callee.type === "Identifier" && "name" in callee && typeof callee.name === "string") {
          return { calleeName: callee.name };
        }
        if (
          callee.type === "MemberExpression" &&
          "property" in callee &&
          callee.property &&
          typeof callee.property === "object"
        ) {
          if (
            "type" in callee.property &&
            callee.property.type === "Identifier" &&
            "name" in callee.property &&
            typeof callee.property.name === "string"
          ) {
            return { calleeName: callee.property.name };
          }
        }
      }
    }
  }

  return null;
}

export default {
  meta: {
    type: "suggestion",
    docs: {
      description:
        "Prohibit calling configured functions and directly accessing their return value (e.g. useRuntimeConfig().public.apiBase).",
      recommended: false,
    },
    schema: [
      {
        type: "object",
        properties: {
          functions: {
            type: "array",
            items: {
              type: "string",
            },
          },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context: Context) {
    const rawOptions = context.options[0];
    const options =
      typeof rawOptions === "object" && rawOptions !== null
        ? (rawOptions as { functions?: string[] })
        : undefined;

    const targetFunctions = new Set<string>(options?.functions ?? defaultTargetFunctions);

    return {
      MemberExpression(node) {
        const callInfo = unwrapCallExpression(node.object);
        if (!callInfo) {
          return;
        }

        if (targetFunctions.has(callInfo.calleeName)) {
          context.report({
            node,
            message: `Direct member access on '${callInfo.calleeName}()' is prohibited. Assign the return value to a variable or destructure it first.`,
          });
        }
      },
    };
  },
} satisfies Rule;

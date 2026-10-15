import type { Context, Rule, Variable } from "./types.ts";
import { isErrorAllowed } from "./utils.ts";

export default {
  meta: {
    type: "suggestion",
    docs: {
      description:
        "Enforce canonical error naming conventions ('err', or ending with 'Err') in catch clauses.",
      recommended: false,
    },
    schema: [
      {
        type: "object",
        properties: {
          exceptions: {
            type: "array",
            items: {
              type: "string",
            },
            default: ["_"],
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
        ? (rawOptions as { exceptions?: string[] })
        : undefined;

    const exceptions = new Set(options?.exceptions ?? ["_"]);

    function checkCatchVariable(variable: Variable): void {
      const name = variable.name;
      if (!name || exceptions.has(name)) {
        return;
      }

      if (!isErrorAllowed(name)) {
        for (const identifier of variable.identifiers) {
          context.report({
            node: identifier,
            message: `'${name}' should be named 'err', suffixed with 'Err', or prefixed with 'Err'`,
          });
        }
      }
    }

    return {
      CatchClause(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkCatchVariable(variable);
        }
      },
    };
  },
} satisfies Rule;

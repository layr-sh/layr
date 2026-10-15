import type { Context, Rule, Variable } from "./types.ts";
import { hasProhibitedSuffix } from "./utils.ts";

export default {
  meta: {
    type: "suggestion",
    docs: {
      description:
        "Disallow Hungarian notation type representation suffixes (e.g. Bool, Boolean, Str, Int) on variables and arguments.",
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

    function checkVariable(variable: Variable): void {
      const name = variable.name;
      if (!name || exceptions.has(name)) {
        return;
      }

      if (hasProhibitedSuffix(name)) {
        for (const identifier of variable.identifiers) {
          context.report({
            node: identifier,
            message: `meaningless representation suffix prohibited: ${name}`,
          });
        }
      }
    }

    return {
      VariableDeclarator(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkVariable(variable);
        }
      },
      FunctionDeclaration(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkVariable(variable);
        }
      },
      FunctionExpression(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkVariable(variable);
        }
      },
      ArrowFunctionExpression(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkVariable(variable);
        }
      },
    };
  },
} satisfies Rule;

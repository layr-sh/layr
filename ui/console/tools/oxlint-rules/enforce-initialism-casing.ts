import type { Context, Rule, Variable } from "./types.ts";
import { checkJsInitialismCasing } from "./utils.ts";

export default {
  meta: {
    type: "suggestion",
    docs: {
      description:
        "Enforce standard JavaScript camelCase casing for acronyms (e.g. userId instead of userID, serverUrl instead of serverURL).",
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

      const result = checkJsInitialismCasing(name);
      if (result) {
        for (const identifier of variable.identifiers) {
          context.report({
            node: identifier,
            message: `disallowed uppercase initialism "${result.word}" in "${name}". Use "${result.expected}" instead.`,
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

import type { Context, Rule, Variable } from "./types.ts";

export default {
  meta: {
    type: "suggestion",
    docs: {
      description: "Disallow variable and argument names shorter than a specified minimum length.",
      recommended: false,
    },
    schema: [
      {
        type: "object",
        properties: {
          min: {
            type: "integer",
            minimum: 1,
            default: 3,
          },
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
    const options = context.options[0] as { min?: number; exceptions?: string[] } | undefined;
    const min = options?.min ?? 3;
    const exceptions = new Set(options?.exceptions ?? ["_"]);

    function checkVariable(variable: Variable): void {
      const name = variable.name;
      if (!name || exceptions.has(name) || name.length >= min) {
        return;
      }

      for (const identifier of variable.identifiers) {
        context.report({
          node: identifier,
          message: `Variable or argument '${name}' is too short (${name.length} characters). Minimum allowed length is ${min}.`,
        });
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
          if (variable.defs.some((def) => def.type === "Parameter")) {
            checkVariable(variable);
          }
        }
      },
      FunctionExpression(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          if (variable.defs.some((def) => def.type === "Parameter")) {
            checkVariable(variable);
          }
        }
      },
      ArrowFunctionExpression(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          if (variable.defs.some((def) => def.type === "Parameter")) {
            checkVariable(variable);
          }
        }
      },
      CatchClause(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkVariable(variable);
        }
      },
    };
  },
} satisfies Rule;

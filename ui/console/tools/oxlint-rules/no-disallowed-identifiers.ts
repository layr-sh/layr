import type { Context, Rule, Variable } from "./types.ts";
import { defaultDisallowedAbbreviations, isDisallowedIdentifier } from "./utils.ts";

export default {
  meta: {
    type: "suggestion",
    docs: {
      description: "Disallow lazy abbreviations (e.g. cfg, req, res, val) in identifiers.",
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
          disallowedWords: {
            type: "array",
            items: {
              type: "string",
            },
          },
          allowedWords: {
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
        ? (rawOptions as {
            exceptions?: string[];
            disallowedWords?: string[];
            allowedWords?: string[];
          })
        : undefined;

    const exceptions = new Set(options?.exceptions ?? ["_"]);
    const allowedWordsSet = new Set(options?.allowedWords?.map((word) => word.toLowerCase()) ?? []);
    const disallowedMap: Record<string, string> = {};

    for (const [key, value] of Object.entries(defaultDisallowedAbbreviations)) {
      if (!allowedWordsSet.has(key)) {
        disallowedMap[key] = value;
      }
    }

    if (options?.disallowedWords) {
      for (const word of options.disallowedWords) {
        const lower = word.toLowerCase();
        if (!allowedWordsSet.has(lower)) {
          disallowedMap[lower] = "full descriptive name";
        }
      }
    }

    function checkVariable(variable: Variable): void {
      const name = variable.name;
      if (!name || exceptions.has(name)) {
        return;
      }

      const { isDisallowed, word, suggestion } = isDisallowedIdentifier(name, disallowedMap);
      if (isDisallowed) {
        for (const identifier of variable.identifiers) {
          context.report({
            node: identifier,
            message: `disallowed abbreviation "${word}" in "${name}". ${suggestion}`.trim(),
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
      CatchClause(node) {
        for (const variable of context.sourceCode.getDeclaredVariables(node)) {
          checkVariable(variable);
        }
      },
    };
  },
} satisfies Rule;

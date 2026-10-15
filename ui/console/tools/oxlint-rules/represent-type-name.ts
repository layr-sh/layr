import type { Context, Rule, VariableDeclarator } from "./types.ts";

const defaultGenericOrPrimitiveTypes: ReadonlySet<string> = new Set([
  "string",
  "number",
  "boolean",
  "bigint",
  "symbol",
  "any",
  "unknown",
  "never",
  "void",
  "null",
  "undefined",
  "const",
  "record",
  "array",
  "promise",
  "ref",
  "computedref",
  "writablecomputedref",
  "map",
  "set",
  "readonlyset",
  "readonlyarray",
  "readonlymap",
  "readonly",
  "partial",
  "required",
  "pick",
  "omit",
  "extract",
  "exclude",
  "nonnullable",
  "parameters",
  "returntype",
  "instancetype",
  "awaited",
  "weakmap",
  "weakset",
  "object",
  "function",
  "regexp",
  "date",
  "rule",
  "plugin",
  "component",
  "context",
]);

const defaultExcludedFunctions: ReadonlySet<string> = new Set([
  "ref",
  "shallowRef",
  "reactive",
  "computed",
  "toRef",
  "toRefs",
  "watch",
  "watchEffect",
  "useState",
  "useFetch",
  "useAsyncData",
  "useLazyFetch",
  "useLazyAsyncData",
  "useNuxtData",
  "createApp",
  "getString",
  "getNumber",
  "getBoolean",
  "getData",
  "getFullYear",
  "getUTCFullYear",
  "getMonth",
  "getUTCMonth",
  "getDate",
  "getUTCDate",
  "getDay",
  "getUTCDay",
  "getHours",
  "getUTCHours",
  "getMinutes",
  "getUTCMinutes",
  "getSeconds",
  "getUTCSeconds",
  "getMilliseconds",
  "getUTCMilliseconds",
  "getTime",
  "getTimezoneOffset",
]);

function canonicalVariableName(typeName: string): string {
  if (!typeName) {
    return typeName;
  }
  if (typeName === typeName.toUpperCase()) {
    return typeName.toLowerCase();
  }
  const acronymMatch = typeName.match(/^([A-Z]+)([A-Z][a-z0-9])/);
  if (acronymMatch) {
    const leadingAcronym = acronymMatch[1];
    return leadingAcronym.toLowerCase() + typeName.slice(leadingAcronym.length);
  }
  return typeName[0].toLowerCase() + typeName.slice(1);
}

function isNameMatchingType(name: string, typeName: string): boolean {
  if (!name || !typeName || name === "_") {
    return true;
  }

  const bare = typeName;
  const canonical = canonicalVariableName(bare);

  const nameLower = name.toLowerCase();
  const bareLower = bare.toLowerCase();
  const canonicalLower = canonical.toLowerCase();

  // 1. Exact match (case-insensitive)
  if (nameLower === bareLower || nameLower === canonicalLower) {
    return true;
  }

  // 2. Suffix match (case-insensitive, and allowing plural 's')
  const nameSingularLower = nameLower.endsWith("s") ? nameLower.slice(0, -1) : nameLower;

  if (
    nameLower.endsWith(bareLower) ||
    nameLower.endsWith(canonicalLower) ||
    nameSingularLower.endsWith(bareLower) ||
    nameSingularLower.endsWith(canonicalLower)
  ) {
    return true;
  }

  return false;
}

function isGenericOrPrimitiveType(name: string): boolean {
  return defaultGenericOrPrimitiveTypes.has(name.toLowerCase());
}

function extractTargetTypeName(
  declarator: VariableDeclarator,
  excludedFns: ReadonlySet<string> = defaultExcludedFunctions,
): string | null {
  // 1. Explicit type annotation on the variable, e.g. `const x: RuntimeConfig = ...`
  if (declarator.id && typeof declarator.id === "object" && "typeAnnotation" in declarator.id) {
    const tsAnnotation = declarator.id.typeAnnotation;
    if (tsAnnotation && typeof tsAnnotation === "object" && "typeAnnotation" in tsAnnotation) {
      const typeNode = tsAnnotation.typeAnnotation;
      if (typeNode && typeof typeNode === "object") {
        if ("type" in typeNode && typeNode.type === "TSTypeReference" && "typeName" in typeNode) {
          const typeNameNode = typeNode.typeName;
          if (typeNameNode && typeof typeNameNode === "object" && "name" in typeNameNode) {
            const name = String(typeNameNode.name);
            if (!isGenericOrPrimitiveType(name)) {
              return name;
            }
          }
        }
      }
    }
  }

  const init = declarator.init;
  if (!init || typeof init !== "object") {
    return null;
  }

  // 2. Type assertion, e.g. `useRuntimeConfig() as RuntimeConfig`
  if ("type" in init && (init.type === "TSAsExpression" || init.type === "TSTypeAssertion")) {
    if ("typeAnnotation" in init) {
      const typeNode = init.typeAnnotation;
      if (
        typeNode &&
        typeof typeNode === "object" &&
        "type" in typeNode &&
        typeNode.type === "TSTypeReference" &&
        "typeName" in typeNode
      ) {
        const typeNameNode = typeNode.typeName;
        if (typeNameNode && typeof typeNameNode === "object" && "name" in typeNameNode) {
          const name = String(typeNameNode.name);
          if (!isGenericOrPrimitiveType(name)) {
            return name;
          }
        }
      }
    }
  }

  // 3. Constructor call, e.g. `new S3Client(...)` or `new QueueManager(...)`
  if ("type" in init && init.type === "NewExpression" && "callee" in init) {
    const callee = init.callee;
    if (callee && typeof callee === "object") {
      if ("type" in callee && callee.type === "Identifier" && "name" in callee) {
        const name = String(callee.name);
        if (!isGenericOrPrimitiveType(name)) {
          return name;
        }
      }
      if ("type" in callee && callee.type === "MemberExpression" && "property" in callee) {
        const prop = callee.property;
        if (
          prop &&
          typeof prop === "object" &&
          "type" in prop &&
          prop.type === "Identifier" &&
          "name" in prop
        ) {
          const name = String(prop.name);
          if (!isGenericOrPrimitiveType(name)) {
            return name;
          }
        }
      }
    }
  }

  // 4. Function call, e.g. `useRuntimeConfig()`, `resolveRuntimeConfig()`, `createOpenAI()`
  if ("type" in init && init.type === "CallExpression" && "callee" in init) {
    const callee = init.callee;
    let calleeName = "";
    if (callee && typeof callee === "object") {
      if ("type" in callee && callee.type === "Identifier" && "name" in callee) {
        calleeName = String(callee.name);
      } else if ("type" in callee && callee.type === "MemberExpression" && "property" in callee) {
        const prop = callee.property;
        if (
          prop &&
          typeof prop === "object" &&
          "type" in prop &&
          prop.type === "Identifier" &&
          "name" in prop
        ) {
          calleeName = String(prop.name);
        }
      }
    }

    if (calleeName && !excludedFns.has(calleeName)) {
      const prefixMatch = calleeName.match(/^(?:use|resolve|create|get)([A-Z][a-zA-Z0-9]+)$/);
      if (prefixMatch) {
        const extractedType = prefixMatch[1];
        if (!isGenericOrPrimitiveType(extractedType)) {
          return extractedType;
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
        "Enforce that variables represent their actual type name (e.g. const runtimeConfig = useRuntimeConfig() or const appRuntimeConfig = useRuntimeConfig()).",
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
          excludedFunctions: {
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
            excludedFunctions?: string[];
          })
        : undefined;

    const exceptions = new Set(options?.exceptions ?? ["_"]);
    const excludedFns = new Set(defaultExcludedFunctions);
    if (options?.excludedFunctions) {
      for (const funcName of options.excludedFunctions) {
        excludedFns.add(funcName);
      }
    }

    return {
      VariableDeclarator(node) {
        // Only inspect single-binding identifier variable declarations (e.g. const x = ...)
        if (
          !node.id ||
          typeof node.id !== "object" ||
          !("name" in node.id) ||
          typeof node.id.name !== "string"
        ) {
          return;
        }

        const varName = node.id.name;
        if (!varName || exceptions.has(varName)) {
          return;
        }

        const targetType = extractTargetTypeName(node, excludedFns);
        if (!targetType) {
          return;
        }

        if (!isNameMatchingType(varName, targetType)) {
          const canonical = canonicalVariableName(targetType);
          context.report({
            node: node.id,
            message: `'${varName}' should be named '${canonical}' or suffixed with '${targetType}'`,
          });
        }
      },
    };
  },
} satisfies Rule;

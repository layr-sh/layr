import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import noDestructuredCallReturn from "../no-destructured-call-return.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "ts",
    },
  },
});

ruleTester.run("no-destructured-call-return", noDestructuredCallReturn, {
  valid: [
    // Assigned to a variable first, then destructured from the variable
    {
      code: "const runtimeConfig = useRuntimeConfig(); const { public: publicConfig } = runtimeConfig;",
    },
    {
      code: "const runtimeConfig = useRuntimeConfig(); const publicConfig = runtimeConfig.public;",
    },
    {
      code: "const nitroRuntimeConfig = resolveRuntimeConfig(event); const openai = nitroRuntimeConfig.openai;",
    },
    // Unconfigured functions are allowed to be destructured
    {
      code: "const { data, pending } = useFetch('/api');",
    },
    {
      code: "const { params } = useRoute();",
    },
    {
      code: "const [state, setState] = useState();",
    },
    // Non-destructured variable declarations are allowed
    {
      code: "const runtimeConfig = useRuntimeConfig();",
    },
  ],
  invalid: [
    // Object destructuring from useRuntimeConfig
    {
      code: "const { public: publicConfig } = useRuntimeConfig();",
      errors: [
        {
          message:
            "Destructuring return value of 'useRuntimeConfig()' is prohibited. Assign it to a variable first.",
        },
      ],
    },
    {
      code: "const { public, app } = useRuntimeConfig();",
      errors: [
        {
          message:
            "Destructuring return value of 'useRuntimeConfig()' is prohibited. Assign it to a variable first.",
        },
      ],
    },
    // Array destructuring from useRuntimeConfig
    {
      code: "const [a, b] = useRuntimeConfig();",
      errors: [
        {
          message:
            "Destructuring return value of 'useRuntimeConfig()' is prohibited. Assign it to a variable first.",
        },
      ],
    },
    // Destructuring from resolveRuntimeConfig
    {
      code: "const { openai } = resolveRuntimeConfig(event);",
      errors: [
        {
          message:
            "Destructuring return value of 'resolveRuntimeConfig()' is prohibited. Assign it to a variable first.",
        },
      ],
    },
    // AssignmentExpression destructuring
    {
      code: "let publicConfig; ({ public: publicConfig } = useRuntimeConfig());",
      errors: [
        {
          message:
            "Destructuring return value of 'useRuntimeConfig()' is prohibited. Assign it to a variable first.",
        },
      ],
    },
    // Custom configured function
    {
      code: "const { token } = getAuthSession();",
      options: [{ functions: ["getAuthSession"] }],
      errors: [
        {
          message:
            "Destructuring return value of 'getAuthSession()' is prohibited. Assign it to a variable first.",
        },
      ],
    },
  ],
});

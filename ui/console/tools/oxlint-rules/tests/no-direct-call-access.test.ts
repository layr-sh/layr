import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import noDirectCallAccess from "../no-direct-call-access.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "ts",
    },
  },
});

ruleTester.run("no-direct-call-access", noDirectCallAccess, {
  valid: [
    // Assigned to a variable first
    {
      code: "const runtimeConfig = useRuntimeConfig(); const api = runtimeConfig.public.apiBase;",
    },
    // Destructured from call directly
    {
      code: "const { public: publicConfig } = useRuntimeConfig();",
    },
    // Assigned to a variable first from resolveRuntimeConfig
    {
      code: "const nitroRuntimeConfig = resolveRuntimeConfig(event); const key = nitroRuntimeConfig.openai;",
    },
    // Passed as an argument
    {
      code: "setupService(useRuntimeConfig());",
    },
    // Returned directly
    {
      code: "function getConfig() { return useRuntimeConfig(); }",
    },
    // Unconfigured functions are allowed
    {
      code: "const id = useRoute().params.id;",
    },
    {
      code: "const user = getUser().profile.name;",
    },
  ],
  invalid: [
    // Direct property access
    {
      code: "const api = useRuntimeConfig().public;",
      errors: [
        {
          message:
            "Direct member access on 'useRuntimeConfig()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
    // Deep chained property access
    {
      code: "const api = useRuntimeConfig().accessing.value.directlyHere;",
      errors: [
        {
          message:
            "Direct member access on 'useRuntimeConfig()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
    // Bracket index access
    {
      code: "const api = useRuntimeConfig()['public'];",
      errors: [
        {
          message:
            "Direct member access on 'useRuntimeConfig()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
    // Optional chaining
    {
      code: "const api = useRuntimeConfig()?.public;",
      errors: [
        {
          message:
            "Direct member access on 'useRuntimeConfig()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
    // Parenthesized call
    {
      code: "const api = (useRuntimeConfig()).public;",
      errors: [
        {
          message:
            "Direct member access on 'useRuntimeConfig()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
    // resolveRuntimeConfig direct access
    {
      code: "const apiKey = resolveRuntimeConfig(event).openai.apiKey;",
      errors: [
        {
          message:
            "Direct member access on 'resolveRuntimeConfig()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
    // Custom configured function
    {
      code: "const token = getAuthSession().token;",
      options: [{ functions: ["getAuthSession"] }],
      errors: [
        {
          message:
            "Direct member access on 'getAuthSession()' is prohibited. Assign the return value to a variable or destructure it first.",
        },
      ],
    },
  ],
});

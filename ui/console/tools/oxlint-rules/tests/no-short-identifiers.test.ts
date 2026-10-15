import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import noShortIdentifiers from "../no-short-identifiers.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "js",
    },
  },
});

ruleTester.run("no-short-identifiers", noShortIdentifiers, {
  valid: [
    // 3 or more letters are valid
    { code: "const totalCount = 100;" },
    { code: "let user = 'alice';" },
    { code: "var message = 'hello';" },
    { code: "function calculateTotal(amount, discount) { return amount - discount; }" },
    { code: "const add = (first, second) => first + second;" },
    { code: "for (let item of items) {}" },
    { code: "for (const element in object) {}" },
    { code: "try {} catch (error) {}" },

    // Destructuring: variables assigned are checked, but property keys are not
    { code: "const { id: identifier, db: database } = config;" },
    { code: "const [firstElement, secondElement] = pair;" },

    // Object literals should not be affected (properties are not variables)
    { code: "const config = { x: 1, y: 2, id: '123', db: 'postgres' };" },
    { code: "const result = user.id;" },

    // Default exception: '_' is allowed
    { code: "const _ = 123;" },
    { code: "function handle(_, event) { return event; }" },
    { code: "const [_, value] = pair;" },

    // Custom options: min length and custom exceptions
    {
      code: "const id = 1;",
      options: [{ min: 3, exceptions: ["id"] }],
    },
    {
      code: "for (let i = 0; i < 10; i++) {}",
      options: [{ min: 3, exceptions: ["i"] }],
    },
    {
      code: "const ab = 1;",
      options: [{ min: 2 }],
    },
  ],
  invalid: [
    // 1-2 letter variable declarations
    {
      code: "const x = 1;",
      errors: [
        {
          message:
            "Variable or argument 'x' is too short (1 characters). Minimum allowed length is 3.",
        },
      ],
    },
    {
      code: "let ab = 2;",
      errors: [
        {
          message:
            "Variable or argument 'ab' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
    {
      code: "var id = 3;",
      errors: [
        {
          message:
            "Variable or argument 'id' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
    // For loops
    {
      code: "for (let i = 0; i < 10; i++) {}",
      errors: [
        {
          message:
            "Variable or argument 'i' is too short (1 characters). Minimum allowed length is 3.",
        },
      ],
    },
    {
      code: "for (const el of list) {}",
      errors: [
        {
          message:
            "Variable or argument 'el' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
    // Function parameters
    {
      code: "function compute(a, b) { return a + b; }",
      errors: [
        {
          message:
            "Variable or argument 'a' is too short (1 characters). Minimum allowed length is 3.",
        },
        {
          message:
            "Variable or argument 'b' is too short (1 characters). Minimum allowed length is 3.",
        },
      ],
    },
    {
      code: "const handler = (e) => e.target;",
      errors: [
        {
          message:
            "Variable or argument 'e' is too short (1 characters). Minimum allowed length is 3.",
        },
      ],
    },
    // Catch clause parameter
    {
      code: "try {} catch (e) {}",
      errors: [
        {
          message:
            "Variable or argument 'e' is too short (1 characters). Minimum allowed length is 3.",
        },
      ],
    },
    // Destructuring with short bound variable name
    {
      code: "const { identifier: id } = config;",
      errors: [
        {
          message:
            "Variable or argument 'id' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
    {
      code: "const { id } = config;",
      errors: [
        {
          message:
            "Variable or argument 'id' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
    {
      code: "const [a, bb] = pair;",
      errors: [
        {
          message:
            "Variable or argument 'a' is too short (1 characters). Minimum allowed length is 3.",
        },
        {
          message:
            "Variable or argument 'bb' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
    // Parameter with default value
    {
      code: "function test(fn = () => {}) {}",
      errors: [
        {
          message:
            "Variable or argument 'fn' is too short (2 characters). Minimum allowed length is 3.",
        },
      ],
    },
  ],
});

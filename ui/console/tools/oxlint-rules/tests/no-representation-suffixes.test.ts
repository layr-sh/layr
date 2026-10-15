import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import noRepresentationSuffixes from "../no-representation-suffixes.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "js",
    },
  },
});

ruleTester.run("no-representation-suffixes", noRepresentationSuffixes, {
  valid: [
    { code: "const totalCount = 100;" },
    { code: "let userName = 'alice';" },
    { code: "const isValid = true;" },
    { code: "const isActive = false;" },
    // BigInt / BigInteger are allowed (representing JS BigInt type)
    { code: "const totalBigInt = 100n;" },
    { code: "const amountBigInteger = 200n;" },
    { code: "function calculate(total, active) { return total; }" },
    { code: "const _ = 123;" },
    {
      code: "const countInt = 10;",
      options: [{ exceptions: ["countInt"] }],
    },
  ],
  invalid: [
    {
      code: "const countInt = 10;",
      errors: [
        {
          message: "meaningless representation suffix prohibited: countInt",
        },
      ],
    },
    {
      code: "let nameStr = 'Alice';",
      errors: [
        {
          message: "meaningless representation suffix prohibited: nameStr",
        },
      ],
    },
    {
      code: "var validBool = true;",
      errors: [
        {
          message: "meaningless representation suffix prohibited: validBool",
        },
      ],
    },
    {
      code: "const activeBoolean = false;",
      errors: [
        {
          message: "meaningless representation suffix prohibited: activeBoolean",
        },
      ],
    },
    {
      code: "function compute(amountInt) { return amountInt; }",
      errors: [
        {
          message: "meaningless representation suffix prohibited: amountInt",
        },
      ],
    },
    {
      code: "const fn = (readyBool) => readyBool;",
      errors: [
        {
          message: "meaningless representation suffix prohibited: readyBool",
        },
      ],
    },
  ],
});

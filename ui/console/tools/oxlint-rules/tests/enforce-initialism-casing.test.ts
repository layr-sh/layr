import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import enforceInitialismCasing from "../enforce-initialism-casing.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "js",
    },
  },
});

ruleTester.run("enforce-initialism-casing", enforceInitialismCasing, {
  valid: [
    // Standard JS camelCase acronyms (treated as regular words like getElementById)
    { code: "const userId = 'user_123';" },
    { code: "const serverUrl = 'https://api.example.com';" },
    { code: "const tokenTtl = 3600;" },
    { code: "const sessionSid = 'sess_456';" },
    { code: "const authJwt = 'header.payload.signature';" },
    { code: "const id = '123';" },
    { code: "const url = 'https://example.com';" },
    { code: "const httpResponse = fetch('/api');" },
    { code: "function findUser(userId) { return userId; }" },
    { code: "const getUrl = () => 'https://example.com';" },

    // SCREAMING_SNAKE_CASE constants are allowed
    { code: "const MAX_ID = 100;" },
    { code: "const DEFAULT_SERVER_URL = 'http://localhost';" },
    { code: "const DEFAULT_TTL = 3600;" },

    // Exceptions option
    {
      code: "const userID = '123';",
      options: [{ exceptions: ["userID"] }],
    },
  ],
  invalid: [
    {
      code: "const userID = '123';",
      errors: [
        {
          message: 'disallowed uppercase initialism "ID" in "userID". Use "Id" instead.',
        },
      ],
    },
    {
      code: "let serverURL = 'http://localhost';",
      errors: [
        {
          message: 'disallowed uppercase initialism "URL" in "serverURL". Use "Url" instead.',
        },
      ],
    },
    {
      code: "const tokenTTL = 60;",
      errors: [
        {
          message: 'disallowed uppercase initialism "TTL" in "tokenTTL". Use "Ttl" instead.',
        },
      ],
    },
    {
      code: "const sessionSID = 'abc';",
      errors: [
        {
          message: 'disallowed uppercase initialism "SID" in "sessionSID". Use "Sid" instead.',
        },
      ],
    },
    {
      code: "const authJWT = 'token';",
      errors: [
        {
          message: 'disallowed uppercase initialism "JWT" in "authJWT". Use "Jwt" instead.',
        },
      ],
    },
    {
      code: "function fetchUser(userID) {}",
      errors: [
        {
          message: 'disallowed uppercase initialism "ID" in "userID". Use "Id" instead.',
        },
      ],
    },
    {
      code: "const handler = (requestURL) => requestURL;",
      errors: [
        {
          message: 'disallowed uppercase initialism "URL" in "requestURL". Use "Url" instead.',
        },
      ],
    },
    {
      code: "const HTTPResponse = 123;",
      errors: [
        {
          message: 'disallowed uppercase initialism "HTTP" in "HTTPResponse". Use "http" instead.',
        },
      ],
    },
  ],
});

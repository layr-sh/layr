import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import noDisallowedIdentifiers from "../no-disallowed-identifiers.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "js",
    },
  },
});

ruleTester.run("no-disallowed-identifiers", noDisallowedIdentifiers, {
  valid: [
    // Standard clear names
    { code: "const configuration = {};" },
    { code: "const config = {};" },
    { code: "const request = {};" },
    { code: "const response = {};" },
    { code: "const manager = {};" },
    { code: "const serviceAccount = {};" },
    { code: "const user = {};" },
    { code: "const count = 10;" },
    { code: "const index = 0;" },
    { code: "const button = document.createElement('button');" },

    // Legitimate English words containing abbreviation sub-strings
    { code: "const validate = true;" },
    { code: "const value = 42;" },
    { code: "const resource = '/users';" },
    { code: "const channel = createChannel();" },
    { code: "const column = 'email';" },
    { code: "const collection = [];" },
    { code: "const result = calculate();" },

    // Common standard terms allowed in JS/TS
    { code: "const admin = { role: 'admin' };" },
    { code: "const pool = createPool();" },
    { code: "const db = getDatabase();" },
    { code: "const id = '123';" },

    // Custom allowed words option
    {
      code: "const userCfg = loadConfig();",
      options: [{ allowedWords: ["cfg"] }],
    },
    // Exceptions option
    {
      code: "const req = getRequest();",
      options: [{ exceptions: ["req"] }],
    },
  ],
  invalid: [
    {
      code: "const userCfg = {};",
      errors: [
        {
          message:
            'disallowed abbreviation "Cfg" in "userCfg". Use a descriptive name. Did you mean \'config\'?',
        },
      ],
    },
    {
      code: "function handle(authReq) {}",
      errors: [
        {
          message:
            'disallowed abbreviation "Req" in "authReq". Use a descriptive name. Did you mean \'request\'?',
        },
      ],
    },
    {
      code: "const send = (apiResp) => apiResp;",
      errors: [
        {
          message:
            'disallowed abbreviation "Resp" in "apiResp". Use a descriptive name. Did you mean \'response\'?',
        },
      ],
    },
    {
      code: "const userMgr = new Manager();",
      errors: [
        {
          message:
            'disallowed abbreviation "Mgr" in "userMgr". Use a descriptive name. Did you mean \'manager\'?',
        },
      ],
    },
    {
      code: "const saToken = 'service-account';",
      errors: [
        {
          message:
            'disallowed abbreviation "sa" in "saToken". Use a descriptive name. Did you mean \'serviceAccount\'?',
        },
      ],
    },
    {
      code: "const itemVal = 10;",
      errors: [
        {
          message:
            'disallowed abbreviation "Val" in "itemVal". Use a descriptive name. Did you mean \'value\'?',
        },
      ],
    },
    {
      code: "const tableCols = ['a', 'b'];",
      errors: [
        {
          message:
            "disallowed abbreviation \"Cols\" in \"tableCols\". Use a descriptive name. Did you mean 'columns' or 'colors'?",
        },
      ],
    },
    {
      code: "const submitBtn = document.querySelector('button');",
      errors: [
        {
          message:
            'disallowed abbreviation "Btn" in "submitBtn". Use a descriptive name. Did you mean \'button\'?',
        },
      ],
    },
    {
      code: "const totalCnt = 100;",
      errors: [
        {
          message:
            'disallowed abbreviation "Cnt" in "totalCnt". Use a descriptive name. Did you mean \'count\'?',
        },
      ],
    },
    {
      code: "const itemIdx = 3;",
      errors: [
        {
          message:
            'disallowed abbreviation "Idx" in "itemIdx". Use a descriptive name. Did you mean \'index\'?',
        },
      ],
    },
    {
      code: "const consoleUsr = 'alice';",
      errors: [
        {
          message:
            'disallowed abbreviation "Usr" in "consoleUsr". Use a descriptive name. Did you mean \'user\'?',
        },
      ],
    },
    {
      code: "const erFollowers = 0;",
      options: [{ disallowedWords: ["er"] }],
      errors: [
        {
          message: 'disallowed abbreviation "er" in "erFollowers". full descriptive name',
        },
      ],
    },
    {
      code: "const totalERReachSum = 0;",
      options: [{ disallowedWords: ["er"] }],
      errors: [
        {
          message: 'disallowed abbreviation "ER" in "totalERReachSum". full descriptive name',
        },
      ],
    },
    {
      code: "const acceptedSubs = [];",
      errors: [
        {
          message:
            "disallowed abbreviation \"Subs\" in \"acceptedSubs\". Use a descriptive name. Did you mean 'subjects' or 'subscriptions'?",
        },
      ],
    },
    {
      code: "const sub = {};",
      errors: [
        {
          message:
            "disallowed abbreviation \"sub\" in \"sub\". Use a descriptive name. Did you mean 'subject' or 'subscription'?",
        },
      ],
    },
    // Custom disallowed words option
    {
      code: "const tempBuffer = [];",
      options: [{ disallowedWords: ["buffer"] }],
      errors: [
        {
          message: 'disallowed abbreviation "Buffer" in "tempBuffer". full descriptive name',
        },
      ],
    },
  ],
});

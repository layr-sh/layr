import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import enforceErrorNaming from "../enforce-error-naming.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "js",
    },
  },
});

ruleTester.run("enforce-error-naming", enforceErrorNaming, {
  valid: [
    { code: "try {} catch (err) {}" },
    { code: "try {} catch (dbErr) {}" },
    { code: "try {} catch (authErr) {}" },
    { code: "try {} catch (parseErr) {}" },
    { code: "try {} catch (ErrNotFound) {}" },
    { code: "try {} catch (_) {}" },
    {
      code: "try {} catch (error) {}",
      options: [{ exceptions: ["error"] }],
    },
  ],
  invalid: [
    {
      code: "try {} catch (e) {}",
      errors: [
        {
          message: "'e' should be named 'err', suffixed with 'Err', or prefixed with 'Err'",
        },
      ],
    },
    {
      code: "try {} catch (error) {}",
      errors: [
        {
          message: "'error' should be named 'err', suffixed with 'Err', or prefixed with 'Err'",
        },
      ],
    },
    {
      code: "try {} catch (myError) {}",
      errors: [
        {
          message: "'myError' should be named 'err', suffixed with 'Err', or prefixed with 'Err'",
        },
      ],
    },
    {
      code: "try {} catch (exception) {}",
      errors: [
        {
          message: "'exception' should be named 'err', suffixed with 'Err', or prefixed with 'Err'",
        },
      ],
    },
  ],
});

import { describe, expect, it } from "vitest";
import { Buffer } from "node:buffer";
import {
  readFunctionArchive,
  resolveFunctionDeployment,
  validFunctionDate,
  validFunctionDomain,
  validFunctionFilePath,
} from "../../utils/function-forms";

describe("Function form validation", () => {
  it.each(["index.js", "lib/helper.js", "a-b/file.ts"])(
    "accepts relative bundle path %s",
    (value) => {
      expect(validFunctionFilePath(value)).toBe(true);
    },
  );
  it.each([
    "",
    "/absolute.js",
    "../escape.js",
    "lib/../escape.js",
    "./index.js",
    "lib//file.js",
    "lib\\file.js",
  ])("rejects unsafe bundle path %s", (value) => {
    expect(validFunctionFilePath(value)).toBe(false);
  });
  it.each(["api.example.com", "EXAMPLE.COM", "127.0.0.1"])(
    "accepts backend-compatible domain %s",
    (value) => {
      expect(validFunctionDomain(value)).toBe(true);
    },
  );
  it.each([
    "",
    "localhost",
    "https://example.com",
    "-bad.example.com",
    "example..com",
    "bad_.example.com",
    `${"a".repeat(64)}.com`,
    `${"a.".repeat(130)}com`,
  ])("rejects malformed domain %s", (value) => {
    expect(validFunctionDomain(value)).toBe(false);
  });
  it.each(["2026-08-04", "2024-02-29"])("accepts compatibility date %s", (value) => {
    expect(validFunctionDate(value)).toBe(true);
  });
  it.each(["", "not-a-date", "2026-99-99", "2026-02-30"])(
    "rejects compatibility date %s",
    (value) => {
      expect(validFunctionDate(value)).toBe(false);
    },
  );
  it("encodes binary archive bytes without corrupting large chunks", async () => {
    const uint8Array = Uint8Array.from({ length: 65537 }, (_, index) => index % 256);
    const file = new File([uint8Array.buffer], "bundle.ZIP");
    expect(await readFunctionArchive(file, file.size)).toEqual({
      name: file.name,
      size: file.size,
      format: "zip",
      content: Buffer.from(uint8Array).toString("base64"),
    });
    expect((await readFunctionArchive(new File(["tar bytes"], "bundle.tar"))).format).toBe("tar");
  });
  it.each(["bundle", "bundle.js", "bundle.tgz", "bundle.tar.gz"])(
    "rejects unsupported archive %s",
    async (name) => {
      await expect(readFunctionArchive(new File(["bytes"], name))).rejects.toThrow("Choose a ZIP");
    },
  );
  it("rejects empty and oversized archives before reading them", async () => {
    await expect(readFunctionArchive(new File([], "empty.zip"))).rejects.toThrow(
      "archive is empty",
    );
    await expect(readFunctionArchive(new File(["four"], "bundle.zip"), 3)).rejects.toThrow(
      "service limit",
    );
  });
  it("serializes archives separately from source files and supports inherited or explicit flags", () => {
    const form = {
      bundleSource: "archive" as const,
      archive: { name: "bundle.zip", size: 1, format: "zip" as const, content: "AA==" },
      files: [],
      variables: [],
      overrideRuntime: true,
      overrideFlags: false,
      compatibilityDate: "2026-08-04",
      compatibilityFlags: [" nodejs_compat ", "nodejs_compat"],
    };
    expect(resolveFunctionDeployment(form, "index.js")).toEqual({
      errors: [],
      input: {
        bundleFormat: "zip",
        bundleContent: "AA==",
        environmentVariables: {},
        workerdRuntimeConfig: { compatibilityDate: "2026-08-04" },
      },
    });
    form.overrideFlags = true;
    expect(
      resolveFunctionDeployment(form, "index.js").input.workerdRuntimeConfig?.compatibilityFlags,
    ).toEqual(["nodejs_compat"]);
    form.compatibilityFlags = ["bad flag"];
    expect(resolveFunctionDeployment(form, "index.js").errors).toContainEqual(
      expect.objectContaining({ name: "compatibilityFlags" }),
    );
    expect(resolveFunctionDeployment({ ...form, archive: null }, "index.js").errors).toContainEqual(
      expect.objectContaining({ name: "archive" }),
    );
    expect(
      resolveFunctionDeployment({ ...form, archive: { ...form.archive, content: "" } }, "index.js")
        .errors,
    ).toContainEqual(expect.objectContaining({ name: "archive" }));
  });
  it("summarizes invalid paths even when the entrypoint itself is valid", () => {
    const form = {
      files: [
        { path: "index.js", content: "ok" },
        { path: "../bad.js", content: "" },
      ],
      variables: [],
      overrideRuntime: false,
      compatibilityDate: "",
      compatibilityFlags: "",
    };
    expect(resolveFunctionDeployment(form, "index.js").errors.map((error) => error.name)).toEqual([
      "files.1.path",
      "files",
    ]);
  });

  it("serializes file trees, variables, and optional runtime overrides", () => {
    const form = {
      files: [
        { path: "index.js", content: "export default {};" },
        { path: "lib/helper.js", content: "" },
      ],
      variables: [{ name: "API_KEY", value: "secret=value" }],
      overrideRuntime: false,
      compatibilityDate: "",
      compatibilityFlags: "",
    };
    expect(resolveFunctionDeployment(form, "index.js")).toEqual({
      errors: [],
      input: {
        bundleFiles: { "index.js": "export default {};", "lib/helper.js": "" },
        environmentVariables: { API_KEY: "secret=value" },
      },
    });
    form.overrideRuntime = true;
    expect(resolveFunctionDeployment(form, "index.js").errors).toContainEqual(
      expect.objectContaining({ name: "compatibilityDate" }),
    );
    form.compatibilityDate = "2026-08-04";
    form.compatibilityFlags = "nodejs_compat,\n test_flag \n";
    expect(resolveFunctionDeployment(form, "index.js").input.workerdRuntimeConfig).toEqual({
      compatibilityDate: "2026-08-04",
      compatibilityFlags: ["nodejs_compat", "test_flag"],
    });
  });
  it("reports duplicate files, missing or empty entrypoints, and invalid environment keys", () => {
    const form = {
      files: [
        { path: "../bad", content: "bad" },
        { path: "index.js", content: "" },
        { path: "index.js", content: "duplicate" },
      ],
      variables: [
        { name: "1_BAD", value: "" },
        { name: "API_KEY", value: "one" },
        { name: "API_KEY", value: "two" },
      ],
      overrideRuntime: false,
      compatibilityDate: "",
      compatibilityFlags: "",
    };
    expect(resolveFunctionDeployment(form, "index.js").errors.map((error) => error.name)).toEqual([
      "files.0.path",
      "files.2.path",
      "files",
      "variables.0.name",
      "variables.2.name",
    ]);
    expect(resolveFunctionDeployment({ ...form, files: [] }, "missing.js").errors).toContainEqual(
      expect.objectContaining({ name: "files" }),
    );
  });
});

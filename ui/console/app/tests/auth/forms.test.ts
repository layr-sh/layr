import { beforeAll, describe, expect, it } from "vitest";
import type { AuthControlPlaneModels as Models } from "layr-client";
import { rootAuth } from "./helpers";
import {
  authConfigurationSections,
  authSignInMethodSections,
  authSecuritySections,
  authHostedSignInSections,
  authEmailSections,
  authSMSSections,
  authEmailTemplates,
  authSMSTemplates,
  authFieldValue,
  setAuthField,
  validateAuthFields,
  resolveAuthUser,
  authUserLabel,
  authUserLocked,
  authConfigInput,
} from "../../utils/auth-forms";
import {
  authRecordFields,
  authRecordIdentity,
  authRecordRows,
  blankAuthRecord,
  resolveAuthRecord,
  updateAuthRecord,
  type AuthRecordKind,
} from "../../utils/auth-records";
let config: Models.Config;
beforeAll(async () => {
  config = await rootAuth.cp.config.get();
});
const userForm = () => ({
  email: " user@example.com ",
  phone: "",
  password: "",
  role: "authenticated",
  emailVerified: false,
  phoneVerified: false,
  properties: "{}",
});

describe("Auth backend-aligned form resolution", () => {
  it("uses SDK field names, accessible descriptions, and nested configuration values", () => {
    for (const section of [
      ...authConfigurationSections,
      ...authSignInMethodSections,
      ...authSecuritySections,
      ...authHostedSignInSections,
      ...authEmailSections,
      ...authSMSSections,
      ...authEmailTemplates,
      ...authSMSTemplates,
    ]) {
      expect(section.description).toBeTruthy();
      for (const field of section.fields) {
        expect(field.description).toBeTruthy();
        expect(field.label).toBeTruthy();
        const parent = field.name.split(".").slice(0, -1).join(".");
        expect(authFieldValue(config, parent)).toBeTypeOf("object");
        if (field.type === "number" && field.min! > 0)
          expect(authFieldValue(config, field.name)).toBeGreaterThanOrEqual(field.min!);
      }
    }
    expect(authFieldValue({ nested: null }, "nested.absent.value")).toBeUndefined();
    const state = { a: { b: 1 } };
    setAuthField(state, "a.b", 2);
    expect(state.a.b).toBe(2);
    setAuthField(state, "a", { b: 3 });
    expect(state.a.b).toBe(3);
  });
  it("validates numeric defaults and required text, arrays, and nullable values", () => {
    const fields = [
      { name: "number", label: "Number", description: "Number", type: "number" as const, min: 1 },
      { name: "text", label: "Text", description: "Text", required: true },
      { name: "tags", label: "Tags", description: "Tags", type: "tags" as const, required: true },
    ];
    for (const value of [0, 1.5, undefined])
      expect(validateAuthFields({ number: value, text: "", tags: [] }, fields)).toHaveLength(3);
    expect(validateAuthFields({ number: 1, text: "yes", tags: ["yes"] }, fields)).toEqual([]);
    expect(validateAuthFields({ number: 1, text: null, tags: null }, fields)).toHaveLength(2);
    expect(validateAuthFields({}, [{ ...fields[0]!, min: 0 }])).toEqual([]);
    expect(validateAuthFields({}, [{ ...fields[0]!, min: 2 }])).toHaveLength(1);
  });
  it("validates application identity and JSON metadata without imposing public sign-up behavior", () => {
    expect(resolveAuthUser(userForm())).toMatchObject({
      errors: [],
      input: { email: "user@example.com", password: "", properties: {} },
    });
    expect(
      resolveAuthUser({
        ...userForm(),
        email: "",
        phone: "+14155552671",
        phoneVerified: true,
        properties: '{"tier":"pro"}',
      }).errors,
    ).toEqual([]);
    for (const email of ["", "invalid"])
      expect(
        resolveAuthUser({ ...userForm(), email }).errors.some((error) => error.name === "email"),
      ).toBe(true);
    expect(
      resolveAuthUser({
        ...userForm(),
        phone: "123",
        role: "",
        email: "",
        emailVerified: true,
      }).errors.map((error) => error.name),
    ).toEqual(["phone", "role", "emailVerified"]);
    expect(resolveAuthUser({ ...userForm(), phoneVerified: true }).errors).toContainEqual(
      expect.objectContaining({ name: "phoneVerified" }),
    );
    for (const properties of ["bad", "null", "[]", '"text"'])
      expect(resolveAuthUser({ ...userForm(), properties }).errors).toContainEqual(
        expect.objectContaining({ name: "properties" }),
      );
    expect(authUserLabel({ id: "id", email: "", phone: "" })).toBe("id");
    expect(authUserLabel({ id: "id", email: "", phone: "+14155552671" })).toBe("+14155552671");
    expect(authUserLabel({ id: "id", email: "user@example.com", phone: "" })).toBe(
      "user@example.com",
    );
    expect(authUserLocked({})).toBe(false);
    expect(authUserLocked({ lockedUntil: new Date(0) })).toBe(false);
    expect(authUserLocked({ lockedUntil: new Date(Date.now() + 60000) })).toBe(true);
  });
  it("preserves configuration and prevents hosted UI from enabling disabled auth methods", () => {
    const input = authConfigInput(config);
    Object.assign(input.password, { enabled: false });
    input.passkeys.enabled = input.emailOtp.enabled = input.smsOtp.enabled = false;
    input.oauthProviders = null;
    input.oidc.ui = {
      showPassword: true,
      showSignUp: true,
      showPasskeys: true,
      showEmailOtp: true,
      showSmsOtp: true,
      showOauth: true,
    };
    expect(Object.values(authConfigInput(input).oidc.ui)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
    ]);
    input.password.enabled =
      input.passkeys.enabled =
      input.emailOtp.enabled =
      input.smsOtp.enabled =
        true;
    input.oauthProviders = { test: { enabled: true, clientId: "client" } };
    expect(Object.values(authConfigInput(input).oidc.ui)).toEqual([
      true,
      true,
      true,
      true,
      true,
      true,
    ]);
    input.oidc.ui = {};
    expect(Object.values(authConfigInput(input).oidc.ui)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
    ]);
  });
  it.each(["provider", "client", "resource"] as const)(
    "resolves %s records and preserves untouched collections",
    (kind) => {
      const input = blankAuthRecord(kind);
      expect(authRecordFields[kind].every((field) => !!field.description)).toBe(true);
      Object.assign(input, {
        key: "custom",
        name: "Example",
        clientId: "example",
        identifier: "urn:example:api",
        redirectUris: [" app://callback ", "app://callback"],
        scopes: [" read ", "read", ""],
        _public: true,
      });
      expect(resolveAuthRecord(kind, input, config, "").errors).toEqual([]);
      const next = updateAuthRecord(kind, config, input, "");
      const key = String(input[authRecordIdentity[kind]]);
      expect(authRecordRows(kind, next).find((row) => row.key === key)).toBeTruthy();
      expect(resolveAuthRecord(kind, input, next, "").errors).toContainEqual(
        expect.objectContaining({ message: "This identifier is already registered." }),
      );
      expect(resolveAuthRecord(kind, input, next, key).errors).toEqual([]);
      const removed = updateAuthRecord(kind, next, { ...input, enabled: false }, key, true);
      if (kind === "provider") expect(removed.oauthProviders!.custom!.enabled).toBe(false);
      else expect(authRecordRows(kind, removed).find((row) => row.key === key)).toBeUndefined();
    },
  );
  it("only updates credentials and enable state for built-in presets", () => {
    const initial = authConfigInput(config);
    initial.oauthProviders = {
      ...initial.oauthProviders,
      google: {
        enabled: false,
        preset: "google",
        clientId: "original",
        scope: "original-scope",
        authUrl: "https://original.example.com/authorize",
      },
    };
    const next = updateAuthRecord(
      "provider",
      initial,
      {
        key: "google",
        preset: "google",
        clientId: "updated",
        clientSecret: "new-secret",
        enabled: true,
        scope: "changed",
        authUrl: "https://changed.example.com/authorize",
        idAttribute: "changed",
      },
      "google",
    );
    expect(next.oauthProviders!.google).toEqual({
      ...initial.oauthProviders.google,
      clientId: "updated",
      clientSecret: "new-secret",
      enabled: true,
    });
    expect(initial.oauthProviders.google!.enabled).toBe(false);
  });
  it("handles missing collections, secret requirements, provider presets, and URL validation", () => {
    const empty = authConfigInput(config);
    empty.oauthProviders = null;
    empty.oidc.clients = empty.oidc.resourceServers = null;
    for (const kind of ["provider", "client", "resource"] as AuthRecordKind[])
      expect(authRecordRows(kind, empty)).toEqual([]);
    expect(
      authRecordRows("provider", {
        ...empty,
        oauthProviders: { enterprise: { enabled: false, preset: "unknown", clientId: "" } },
      })[0]!.name,
    ).toBe("enterprise");
    expect(
      updateAuthRecord(
        "provider",
        empty,
        { key: "google", preset: "google", enabled: false, clientId: "new" },
        "",
      ).oauthProviders!.google,
    ).toMatchObject({ preset: "google", clientId: "new" });
    const provider = {
      ...blankAuthRecord("provider"),
      key: "custom",
      enabled: true,
      authUrl: "bad",
      tokenUrl: "",
      userinfoUrl: "bad",
    };
    expect(
      resolveAuthRecord("provider", provider, empty, "").errors.map((error) => error.name),
    ).toEqual(["clientId", "clientSecret", "tokenUrl", "authUrl", "userinfoUrl"]);
    Object.assign(provider, {
      key: "google",
      clientId: "client",
      clientSecret: "secret",
      authUrl: "https://example.com/auth",
      tokenUrl: "https://example.com/token",
      userinfoUrl: "https://example.com/user",
    });
    expect(resolveAuthRecord("provider", provider, empty, "").errors).toEqual([]);
    Object.assign(provider, {
      key: "custom",
      preset: "github",
      clientSecret: "",
      clientSecretConfigured: true,
    });
    expect(resolveAuthRecord("provider", provider, empty, "").errors).toEqual([]);
    for (const key of ["constructor", "prototype", "__proto__", "BAD", ""])
      expect(
        resolveAuthRecord("provider", { ...provider, key }, empty, "").errors.some(
          (error) => error.name === "key",
        ),
      ).toBe(true);
    const application = {
      ...blankAuthRecord("client"),
      name: "App",
      clientId: "app",
      _public: false,
      redirectUris: ["relative"],
      postSignOutRedirectUris: ["https://example.com/#fragment"],
      backchannelSignOutUri: "bad",
      frontchannelSignOutUri: "bad",
    };
    expect(
      resolveAuthRecord("client", application, empty, "").errors.map((error) => error.name),
    ).toEqual([
      "clientSecret",
      "redirectUris",
      "postSignOutRedirectUris",
      "backchannelSignOutUri",
      "frontchannelSignOutUri",
    ]);
    Object.assign(application, {
      clientSecretConfigured: true,
      redirectUris: ["https://example.com/callback"],
      postSignOutRedirectUris: [],
      backchannelSignOutUri: "https://example.com/back",
      frontchannelSignOutUri: "https://example.com/front",
    });
    expect(resolveAuthRecord("client", application, empty, "").errors).toEqual([]);
    const rowConfig = updateAuthRecord("client", empty, application, "");
    expect(authRecordRows("client", rowConfig)[0]!.status).toBe("Confidential");
    const sparse = { key: "new", name: "API", identifier: "urn:api", scopes: null };
    expect(resolveAuthRecord("resource", sparse, empty, "").input.scopes).toEqual([]);
    expect(
      authRecordRows("resource", updateAuthRecord("resource", empty, sparse, "")),
    ).toHaveLength(1);
    expect(
      authRecordRows("client", updateAuthRecord("client", empty, application, "", true)),
    ).toEqual([]);
    expect(
      authRecordRows("resource", updateAuthRecord("resource", empty, sparse, "", true)),
    ).toEqual([]);
  });
});

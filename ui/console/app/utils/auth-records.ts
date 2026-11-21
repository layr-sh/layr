import type { AuthControlPlaneModels as Models } from "layr-client";
import { validateAuthFields, type AuthField } from "./auth-forms";
export type AuthRecordKind = "provider" | "client" | "resource";
export const authProviderTypes = [
  { label: "Google", value: "google" },
  { label: "GitHub", value: "github" },
  { label: "Apple", value: "apple" },
  { label: "Discord", value: "discord" },
  { label: "Custom OAuth Provider", value: "custom" },
];
export const authRecordIdentity = {
  provider: "key",
  client: "clientId",
  resource: "identifier",
} as const;
export const authRecordFields: Record<AuthRecordKind, AuthField[]> = {
  provider: [
    {
      name: "key",
      label: "Provider ID",
      description: "Stable identifier used by SDK OAuth calls and in this provider's callback URL.",
      required: true,
    },
    {
      name: "enabled",
      label: "Enable Provider",
      type: "switch",
      description: "Allow new sign-ins through this external OAuth provider.",
    },
    {
      name: "preset",
      label: "Provider Preset",
      type: "select",
      options: ["", "google", "github", "apple", "discord", "custom"],
      emptyLabel: "Automatic (from Provider Key)",
      description:
        "Use backend defaults for a built-in provider, or supply custom endpoints below.",
    },
    {
      name: "clientId",
      label: "OAuth Client ID",
      description: "Client identifier issued by the external provider.",
    },
    {
      name: "clientSecret",
      label: "OAuth Client Secret",
      type: "password",
      configured: "clientSecretConfigured",
      description:
        "Leave blank to preserve the saved secret. Secrets cannot be retrieved or cleared here.",
    },
    {
      name: "scope",
      label: "OAuth Scopes",
      description: "Space-separated provider scopes; blank uses preset defaults.",
    },
    {
      name: "authUrl",
      label: "Authorization URL",
      description: "Override the provider's HTTP or HTTPS authorization endpoint.",
    },
    {
      name: "tokenUrl",
      label: "Token URL",
      description: "Override the provider's HTTP or HTTPS token endpoint.",
    },
    {
      name: "userinfoUrl",
      label: "User Info URL",
      description: "Override the provider's HTTP or HTTPS profile endpoint.",
    },
    {
      name: "responseType",
      label: "OAuth Response Type",
      description: "Blank uses preset behavior, normally code.",
    },
    {
      name: "responseMode",
      label: "OAuth Response Mode",
      description: "Blank uses preset behavior; Apple uses form_post.",
    },
    {
      name: "idAttribute",
      label: "Profile ID Attribute",
      description: "Provider profile field containing the stable external user ID.",
    },
    {
      name: "emailAttribute",
      label: "Profile Email Attribute",
      description: "Provider profile field containing the email address.",
    },
    {
      name: "nameAttribute",
      label: "Profile Name Attribute",
      description: "Provider profile field containing the display name.",
    },
    {
      name: "avatarAttribute",
      label: "Profile Avatar Attribute",
      description: "Provider profile field containing the avatar URL.",
    },
  ],
  client: [
    {
      name: "name",
      label: "Application Name",
      description: "Display name for this OIDC relying-party application.",
      required: true,
    },
    {
      name: "clientId",
      label: "OIDC Client ID",
      description: "Stable client identifier. Create a new application to change it.",
      required: true,
    },
    {
      name: "_public",
      label: "Public Client",
      type: "switch",
      description:
        "Browser and mobile apps cannot keep a secret. All applications require S256 PKCE; confidential server apps additionally require a secret.",
    },
    {
      name: "clientSecret",
      label: "OIDC Client Secret",
      type: "password",
      configured: "clientSecretConfigured",
      description:
        "Required for a new confidential client. Leave blank to preserve an existing secret.",
    },
    {
      name: "redirectUris",
      label: "Redirect URIs",
      type: "tags",
      description:
        "Exact absolute callback URIs allowed for this application. Press Enter after each URI.",
      required: true,
    },
    {
      name: "postSignOutRedirectUris",
      label: "Post Sign-Out Redirect URIs",
      type: "tags",
      description: "Exact return URIs allowed after hosted sign-out.",
    },
    {
      name: "scopes",
      label: "OIDC Scopes",
      type: "tags",
      description:
        "Scopes this application may request. An empty list uses the standard OIDC scopes.",
    },
    {
      name: "backchannelSignOutUri",
      label: "Back-Channel Sign-Out URI",
      description: "Server callback for federated sign-out notifications.",
    },
    {
      name: "backchannelSignOutSessionRequired",
      label: "Require Back-Channel Session ID",
      type: "switch",
      description:
        "Include the session requirement in the application's back-channel sign-out configuration.",
    },
    {
      name: "frontchannelSignOutUri",
      label: "Front-Channel Sign-Out URI",
      description: "Browser endpoint used during federated sign-out.",
    },
    {
      name: "frontchannelSignOutSessionRequired",
      label: "Require Front-Channel Session ID",
      type: "switch",
      description: "Include the session requirement in front-channel sign-out requests.",
    },
  ],
  resource: [
    {
      name: "name",
      label: "API Name",
      description: "Display name of the API receiving machine-to-machine tokens.",
      required: true,
    },
    {
      name: "identifier",
      label: "Audience Identifier",
      description: "Exact audience requested by Service Accounts when obtaining M2M tokens.",
      required: true,
    },
    {
      name: "description",
      label: "Description",
      type: "textarea",
      description: "Optional description of this API and its purpose.",
    },
    {
      name: "scopes",
      label: "Resource Scopes",
      type: "tags",
      description:
        "Scopes exposed by this audience, for example invoices:read. These do not create Service Accounts.",
    },
  ],
};
export function authRecordRows(kind: AuthRecordKind, config: Models.Config) {
  if (kind === "provider")
    return Object.entries(config.oauthProviders || {}).map(([key, value]) => ({
      key,
      name:
        authProviderTypes.find((provider) => provider.value === (value.preset || key))?.label ||
        key,
      detail: key,
      status: value.enabled
        ? "Enabled"
        : value.clientId && value.clientSecretConfigured
          ? "Disabled"
          : "Not Configured",
      value,
    }));
  if (kind === "client")
    return (config.oidc.clients || []).map((value) => ({
      key: value.clientId,
      name: value.name,
      detail: value.clientId,
      status: value._public ? "Public PKCE" : "Confidential",
      value,
    }));
  return (config.oidc.resourceServers || []).map((value) => ({
    key: value.identifier,
    name: value.name,
    detail: value.identifier,
    status: "Registered Audience",
    value,
  }));
}
export function blankAuthRecord(kind: AuthRecordKind): Record<string, unknown> {
  const value: Record<string, unknown> = Object.fromEntries(
    authRecordFields[kind].map((field) => [
      field.name,
      field.type === "switch" ? false : field.type === "tags" ? [] : "",
    ]),
  );
  if (kind === "client") {
    value._public = true;
    value.scopes = ["openid", "profile", "email"];
  }
  return value;
}
export function resolveAuthRecord(
  kind: AuthRecordKind,
  form: Record<string, unknown>,
  config: Models.Config,
  editingKey: string,
) {
  const input = { ...form };
  for (const field of authRecordFields[kind]) {
    if (field.type === "tags")
      input[field.name] = [
        ...new Set(
          ((input[field.name] as string[]) || []).map((value) => value.trim()).filter(Boolean),
        ),
      ];
    else if (typeof input[field.name] === "string")
      input[field.name] = (input[field.name] as string).trim();
  }
  const errors = validateAuthFields(input, authRecordFields[kind]);
  const key = String(input[authRecordIdentity[kind]]);
  if (authRecordRows(kind, config).some((row) => row.key === key && row.key !== editingKey))
    errors.push({
      name: authRecordIdentity[kind],
      message: "This identifier is already registered.",
    });
  if (kind === "provider") {
    if (!/^[a-z][a-z0-9_-]*$/.test(key) || ["constructor", "prototype", "__proto__"].includes(key))
      errors.push({
        name: "key",
        message: "Use a lowercase provider key with letters, numbers, underscores, or hyphens.",
      });
    if (input.enabled && !input.clientId)
      errors.push({
        name: "clientId",
        message: "Enter the provider's client ID before enabling it.",
      });
    if (input.enabled && !input.clientSecret && !input.clientSecretConfigured)
      errors.push({
        name: "clientSecret",
        message: "Enter the provider secret before enabling it.",
      });
    if (
      input.enabled &&
      !["google", "github", "apple", "discord"].includes(String(input.preset || key))
    )
      for (const name of ["authUrl", "tokenUrl"])
        if (!input[name])
          errors.push({ name, message: "Custom providers require authorization and token URLs." });
    for (const name of ["authUrl", "tokenUrl", "userinfoUrl"])
      if (input[name] && !/^https?:\/\/[^\s]+$/.test(String(input[name])))
        errors.push({ name, message: "Enter an HTTP or HTTPS URL." });
  }
  if (kind === "client") {
    if (!input._public && !input.clientSecret && !input.clientSecretConfigured)
      errors.push({
        name: "clientSecret",
        message: "Confidential clients require a client secret.",
      });
    for (const name of [
      "redirectUris",
      "postSignOutRedirectUris",
      "backchannelSignOutUri",
      "frontchannelSignOutUri",
    ]) {
      const values = Array.isArray(input[name])
        ? (input[name] as string[])
        : input[name]
          ? [String(input[name])]
          : [];
      if (values.some((value) => !/^[a-z][a-z\d+.-]*:\S+$/i.test(value) || value.includes("#")))
        errors.push({ name, message: "Use absolute URIs without fragments." });
    }
  }
  return { input, errors };
}
export function updateAuthRecord(
  kind: AuthRecordKind,
  config: Models.Config,
  input: Record<string, unknown>,
  editingKey: string,
  remove = false,
): Models.Config {
  const next = JSON.parse(JSON.stringify(config)) as Models.Config;
  if (kind === "provider") {
    const { key, ...provider } = input;
    const preset = String(provider.preset || key);
    const isPreset = authProviderTypes.some(
      (type) => type.value !== "custom" && type.value === preset,
    );
    const savedProvider = isPreset
      ? {
          ...(next.oauthProviders?.[String(key)] || { preset }),
          enabled: provider.enabled,
          clientId: provider.clientId,
          clientSecret: provider.clientSecret,
        }
      : provider;
    next.oauthProviders = {
      ...next.oauthProviders,
      [String(key)]: savedProvider as unknown as Models.OAuthProviderConfig,
    };
  } else if (kind === "client") {
    next.oidc.clients = (next.oidc.clients || []).filter((value) => value.clientId !== editingKey);
    if (!remove) next.oidc.clients.push(input as unknown as Models.OIDCClientConfig);
  } else {
    next.oidc.resourceServers = (next.oidc.resourceServers || []).filter(
      (value) => value.identifier !== editingKey,
    );
    if (!remove) next.oidc.resourceServers.push(input as unknown as Models.ResourceServerConfig);
  }
  return next;
}

import type { FormError } from "@nuxt/ui";
import type { AuthControlPlaneModels as Models } from "layr-client";

export interface AuthField {
  name: string;
  label: string;
  description: string;
  type?: "number" | "switch" | "select" | "multiselect" | "tags" | "textarea" | "password";
  options?: string[];
  choices?: { label: string; value: string }[];
  emptyLabel?: string;
  min?: number;
  required?: boolean;
  configured?: string;
  disabled?: boolean;
  when?: { name: string; value: string | boolean };
}
export interface AuthSection {
  label: string;
  description: string;
  fields: AuthField[];
}
export interface AuthMessageTemplate extends AuthSection {
  messageKind: string;
  variables: string[];
}
export function authFieldValue(state: object, path: string): unknown {
  return path
    .split(".")
    .reduce<unknown>((value, key) => (value as Record<string, unknown> | null)?.[key], state);
}
export function setAuthField(state: object, path: string, value: unknown) {
  const keys = path.split(".");
  const key = keys.pop()!;
  const target = keys.reduce<Record<string, unknown>>(
    (value, part) => value[part] as Record<string, unknown>,
    state as Record<string, unknown>,
  );
  target[key] = value;
}
export function validateAuthFields(state: object, fields: AuthField[]): FormError[] {
  const errors: FormError[] = [];
  for (const field of fields) {
    if (field.when && authFieldValue(state, field.when.name) !== field.when.value) continue;
    const value = authFieldValue(state, field.name);
    if (
      field.type === "number" &&
      !(value == null && field.min === 0) &&
      (!Number.isSafeInteger(value) || (value as number) < field.min!)
    )
      errors.push({ name: field.name, message: `Enter a whole number of at least ${field.min}.` });
    if (
      field.required &&
      (value == null || value === "" || (Array.isArray(value) && !value.length))
    )
      errors.push({ name: field.name, message: `${field.label} is required.` });
  }
  return errors;
}
export function authUserLabel(user: Pick<Models.User, "id" | "email" | "phone">) {
  return user.email || user.phone || user.id;
}
export function authUserLocked(user: Pick<Models.User, "lockedUntil">) {
  return !!user.lockedUntil && user.lockedUntil.getTime() > Date.now();
}
export function authConfigInput(config: Models.Config): Models.Config {
  const input = JSON.parse(JSON.stringify(config)) as Models.Config;
  const hosted = input.oidc.ui;
  hosted.showPassword = !!hosted.showPassword && input.password.enabled;
  hosted.showSignUp = !!hosted.showSignUp && input.password.enabled;
  hosted.showPasskeys = !!hosted.showPasskeys && input.passkeys.enabled;
  hosted.showEmailOtp = !!hosted.showEmailOtp && input.emailOtp.enabled;
  hosted.showSmsOtp = !!hosted.showSmsOtp && input.smsOtp.enabled;
  hosted.showOauth =
    !!hosted.showOauth &&
    Object.values(input.oauthProviders || {}).some((provider) => provider.enabled);
  return input;
}
export function resolveAuthUser(form: {
  email: string;
  phone: string;
  password: string;
  role: string;
  emailVerified: boolean;
  phoneVerified: boolean;
  properties: string;
}) {
  const errors: FormError[] = [];
  const email = form.email.trim().toLowerCase();
  const phone = form.phone.trim().replace(/[\s().-]/g, "");
  if ((!email && !phone) || (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)))
    errors.push({ name: "email", message: "Enter a valid email or an E.164 phone number." });
  if (phone && !/^\+[1-9]\d{7,14}$/.test(phone))
    errors.push({ name: "phone", message: "Use an E.164 phone number, for example +14155552671." });
  if (!form.role.trim()) errors.push({ name: "role", message: "Enter a JWT role." });
  if (form.emailVerified && !email)
    errors.push({ name: "emailVerified", message: "An email is required to mark it verified." });
  if (form.phoneVerified && !phone)
    errors.push({
      name: "phoneVerified",
      message: "A phone number is required to mark it verified.",
    });
  let properties: Record<string, unknown> = {};
  try {
    const parsed: unknown = JSON.parse(form.properties);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error();
    properties = parsed as Record<string, unknown>;
  } catch {
    errors.push({ name: "properties", message: "Enter a JSON object." });
  }
  return {
    errors,
    input: {
      email,
      phone,
      password: form.password,
      role: form.role.trim(),
      emailVerified: form.emailVerified,
      phoneVerified: form.phoneVerified,
      properties,
    },
  };
}

export const authSignInMethodSections: AuthSection[] = [
  {
    label: "Password Authentication",
    description:
      "Password registration, reset policy, and hosted sign-in. These settings do not manage console administrator passwords.",
    fields: [
      {
        name: "password.enabled",
        label: "Enable Password Registration and Reset",
        type: "switch",
        description:
          "Enable password registration, reset, and sign-in through the API and hosted page.",
      },
      {
        name: "password.minLength",
        label: "Minimum Password Length",
        type: "number",
        min: 1,
        description: "Minimum length used by public password registration and password changes.",
      },
      {
        name: "password.requireNumbers",
        label: "Require Numbers",
        type: "switch",
        description: "Require at least one number in new passwords.",
      },
      {
        name: "password.requireSymbols",
        label: "Require Symbols",
        type: "switch",
        description: "Require at least one punctuation or symbol character in new passwords.",
      },
      {
        name: "password.breachCheck.enabled",
        label: "Check Breached Passwords",
        type: "switch",
        description:
          "Check new passwords during API and hosted registration, reset, and password updates using Have I Been Pwned.",
      },
      {
        name: "password.breachCheck.failOpen",
        label: "Allow When Breach Check Is Unavailable",
        type: "switch",
        description: "Allow the request if the external breach-check service cannot be reached.",
      },
    ],
  },
  {
    label: "Passkeys",
    description:
      "WebAuthn credentials are enrolled by application users. Changing the relying party ID can prevent existing passkeys from working.",
    fields: [
      {
        name: "passkeys.enabled",
        label: "Enable Passkeys",
        type: "switch",
        description: "Allow WebAuthn registration and sign-in.",
      },
      {
        name: "passkeys.relyingPartyId",
        label: "Relying Party ID",
        description: "The WebAuthn domain, for example app.example.com; do not include a scheme.",
      },
      {
        name: "passkeys.relyingPartyName",
        label: "Relying Party Name",
        description: "Application name shown by authenticators.",
      },
    ],
  },
  {
    label: "Passwordless and Guest Access",
    description:
      "Let users sign in with an email or SMS code, or start as a guest. Configure your Email or SMS provider before enabling code-based sign-in.",
    fields: [
      {
        name: "emailOtp.enabled",
        label: "Enable Email OTP",
        type: "switch",
        description: "Allow email-code sign-in after email delivery is configured.",
      },
      {
        name: "emailOtp.tokenExpiryMinutes",
        label: "Email OTP Expiry (Minutes)",
        type: "number",
        min: 1,
        description: "How long an email authentication code remains valid.",
      },
      {
        name: "smsOtp.enabled",
        label: "Enable SMS OTP",
        type: "switch",
        description: "Allow phone-code sign-in after SMS delivery is configured.",
      },
      {
        name: "smsOtp.tokenExpiryMinutes",
        label: "SMS OTP Expiry (Minutes)",
        type: "number",
        min: 1,
        description: "How long an SMS authentication code remains valid.",
      },
      {
        name: "anonymous.enabled",
        label: "Enable Anonymous Sign-In",
        type: "switch",
        description: "Allow creation of guest application users without email or phone.",
      },
    ],
  },
];

const mfaAuthSection: AuthSection = {
  label: "Multi-Factor Authentication",
  description:
    "Application-user TOTP policy. Enrollment and challenge verification remain end-user flows, not administrator actions.",
  fields: [
    {
      name: "mfa.enabled",
      label: "Enable Application MFA",
      type: "switch",
      description: "Allow application users to enroll and verify TOTP.",
    },
    {
      name: "mfa.policy",
      label: "MFA Policy",
      type: "select",
      options: ["always", "adaptive"],
      description:
        "Apply the configured MFA policy to enrolled users across API and hosted password, OTP, OAuth, and passkey sign-in.",
    },
    {
      name: "mfa.riskTriggers",
      label: "MFA Risk Triggers",
      type: "multiselect",
      choices: [
        { label: "New Device", value: "new_device" },
        { label: "New IP Address", value: "new_ip" },
        { label: "Excessive Failed Attempts", value: "excessive_failed_attempts" },
        { label: "Critical Failed Attempts", value: "critical_failed_attempts" },
      ],
      description:
        "Additional signals that trigger adaptive MFA. Medium- and high-risk assessments already require a challenge.",
    },
    {
      name: "mfa.issuer",
      label: "MFA Issuer",
      description: "Issuer displayed in users' authenticator apps.",
    },
  ],
};

export const authConfigurationSections: AuthSection[] = [
  {
    label: "Sessions and Tokens",
    description:
      "Session policy values apply to application authentication. Revoking refresh sessions does not retract already issued stateless access JWTs.",
    fields: [
      {
        name: "sessions.accessTokenExpirySeconds",
        label: "Access Token Lifetime (Seconds)",
        type: "number",
        min: 1,
        description: "Lifetime of newly issued application access JWTs.",
      },
      {
        name: "sessions.refreshTokenExpirySeconds",
        label: "Refresh Token Lifetime (Seconds)",
        type: "number",
        min: 1,
        description: "Lifetime used when creating refresh sessions.",
      },
      {
        name: "sessions.idleTimeoutSeconds",
        label: "Session Idle Timeout (Seconds)",
        type: "number",
        min: 1,
        description:
          "Reject sessions after this period without authenticated activity or token refresh.",
      },
      {
        name: "sessions.singleSessionPerUser",
        label: "Single-Session Policy",
        type: "switch",
        description: "Revoke previous sessions when a new session is created for the same user.",
      },
      {
        name: "cache.fastPathSessionsEnabled",
        label: "Cache Refresh Sessions",
        type: "switch",
        description: "Use the key-value store to accelerate refresh-session lookups.",
      },
      {
        name: "cache.sessionTtlSeconds",
        label: "Session Cache TTL (Seconds)",
        type: "number",
        min: 1,
        description: "How long a cached refresh-session lookup remains available.",
      },
    ],
  },
];

export const authSecuritySections: AuthSection[] = [
  mfaAuthSection,
  {
    label: "Attack Protection",
    description: "Sign-in throttling, bot challenges, and new-device notification settings.",
    fields: [
      {
        name: "rateLimiting.enabled",
        label: "Enable Sign-In Rate Limiting",
        type: "switch",
        description:
          "Throttle API and hosted password sign-in attempts by email or phone. OTP has separate fixed limits.",
      },
      {
        name: "rateLimiting.maxSignInAttempts",
        label: "Maximum Sign-In Attempts",
        type: "number",
        min: 1,
        description: "Allowed attempts in the configured time window.",
      },
      {
        name: "rateLimiting.windowDurationSeconds",
        label: "Rate-Limit Window (Seconds)",
        type: "number",
        min: 1,
        description: "Time window used for sign-in attempt counters.",
      },
      {
        name: "rateLimiting.lockoutDurationSeconds",
        label: "Lockout Duration (Seconds)",
        type: "number",
        min: 1,
        description:
          "Block further password sign-in attempts for this duration after the attempt limit is exceeded.",
      },
      {
        name: "threat.botProtection.enabled",
        label: "Enable Bot Protection",
        type: "switch",
        description:
          "Verify CAPTCHA tokens during API and hosted password registration, password sign-in, reset requests, and OTP flows.",
      },
      {
        name: "threat.botProtection.provider",
        label: "CAPTCHA Provider",
        type: "select",
        options: ["turnstile", "recaptcha", "hcaptcha"],
        description: "The external CAPTCHA service used to verify challenge tokens.",
      },
      {
        name: "threat.botProtection.mode",
        label: "CAPTCHA Mode",
        type: "select",
        options: ["always", "adaptive"],
        description: "Always request a challenge or use failed-attempt risk signals.",
      },
      {
        name: "threat.botProtection.siteKey",
        label: "CAPTCHA Site Key",
        description: "Public site key used by your application.",
      },
      {
        name: "threat.botProtection.secretKey",
        label: "CAPTCHA Secret Key",
        type: "password",
        configured: "threat.botProtection.secretKeyConfigured",
        description:
          "Leave blank to preserve the saved secret. Existing secrets cannot be retrieved.",
      },
      {
        name: "threat.botProtection.adaptiveFailedAttempts",
        label: "Adaptive CAPTCHA Threshold",
        type: "number",
        min: 1,
        description: "Failed-attempt threshold for adaptive bot protection.",
      },
      {
        name: "threat.notifyOnNewDevice",
        label: "Notify on New Devices",
        type: "switch",
        description:
          "Send email alerts for new devices during API and hosted password, OTP, OAuth, and passkey sign-in.",
      },
      {
        name: "threat.knownDevicesMaxDays",
        label: "Device History Window (Days)",
        type: "number",
        min: 1,
        description:
          "Compare devices against stored session history within this window. A first sign-in without session history does not trigger a new-device alert.",
      },
    ],
  },
];

export const authHostedSignInSections: AuthSection[] = [
  {
    label: "Hosted Sign-In",
    description:
      "Layr acts as the identity provider for registered applications. Display switches cannot enable a disabled authentication method.",
    fields: [
      {
        name: "oidc.enabled",
        label: "Enable Hosted OIDC Sign-In",
        type: "switch",
        description:
          "Enable hosted authorization and OIDC user-token flows for registered applications. Machine-to-machine token grants are independent of this switch.",
      },
      {
        name: "oidc.ui.logoUrl",
        label: "Hosted Sign-In Logo URL",
        description: "Logo URL used by the hosted sign-in page.",
      },
      {
        name: "oidc.ui.privacyPolicyUrl",
        label: "Privacy Policy URL",
        description: "Privacy-policy link displayed on the hosted sign-in page.",
      },
      {
        name: "oidc.ui.termsOfServiceUrl",
        label: "Terms of Service URL",
        description: "Terms link displayed on the hosted sign-in page.",
      },
      {
        name: "oidc.ui.customCss",
        label: "Hosted Sign-In CSS",
        type: "textarea",
        description:
          "CSS applied by the Auth backend to the hosted sign-in page, not this console.",
      },
      {
        name: "oidc.ui.showPassword",
        label: "Show Password Sign-In",
        type: "switch",
        description: "Display the enabled password method on the hosted page.",
      },
      {
        name: "oidc.ui.showSignUp",
        label: "Show Sign-Up",
        type: "switch",
        description: "Display password registration when password authentication is enabled.",
      },
      {
        name: "oidc.ui.showPasskeys",
        label: "Show Passkeys",
        type: "switch",
        description: "Display passkeys when WebAuthn is enabled.",
      },
      {
        name: "oidc.ui.showOauth",
        label: "Show OAuth Providers",
        type: "switch",
        description: "Display enabled external providers on hosted sign-in.",
      },
      {
        name: "oidc.ui.showEmailOtp",
        label: "Show Email OTP",
        type: "switch",
        description: "Display email codes when the method and delivery are configured.",
      },
      {
        name: "oidc.ui.showSmsOtp",
        label: "Show SMS OTP",
        type: "switch",
        description: "Display SMS codes when the method and delivery are configured.",
      },
    ],
  },
];

export const authEmailSections: AuthSection[] = [
  {
    label: "Email Provider",
    description:
      "SMTP or webhook delivery for verification codes, password reset, OTP, and suspicious-activity notifications. Blank secrets preserve saved values.",
    fields: [
      {
        name: "emailDispatcher.driver",
        label: "Email Provider",
        type: "select",
        options: ["", "smtp", "webhook"],
        description: "Choose a delivery transport, or leave unconfigured.",
      },
      {
        name: "emailDispatcher.senderEmail",
        label: "Sender Email",
        description:
          "From address for authentication email. Defaults to no-reply@layr.sh when blank.",
      },
      {
        name: "emailDispatcher.senderName",
        label: "Sender Name",
        description:
          "From display name. Defaults to the project name, or Layr when no project name is configured.",
      },
      {
        name: "emailDispatcher.smtp.host",
        when: { name: "emailDispatcher.driver", value: "smtp" },
        label: "SMTP Host",
        description: "Hostname of the SMTP server.",
      },
      {
        name: "emailDispatcher.smtp.port",
        when: { name: "emailDispatcher.driver", value: "smtp" },
        label: "SMTP Port",
        type: "number",
        min: 0,
        description: "SMTP TCP port. Required when using SMTP.",
      },
      {
        name: "emailDispatcher.smtp.username",
        when: { name: "emailDispatcher.driver", value: "smtp" },
        label: "SMTP Username",
        description: "Username used to authenticate with SMTP.",
      },
      {
        name: "emailDispatcher.smtp.password",
        when: { name: "emailDispatcher.driver", value: "smtp" },
        label: "SMTP Password",
        type: "password",
        configured: "emailDispatcher.smtp.passwordConfigured",
        description: "Leave blank to preserve the saved SMTP password.",
      },
      {
        name: "emailDispatcher.smtp.tlsMode",
        when: { name: "emailDispatcher.driver", value: "smtp" },
        label: "SMTP TLS Mode",
        type: "select",
        options: ["", "auto", "starttls", "tls", "none"],
        emptyLabel: "Automatic",
        description: "Transport security mode; auto uses the backend's default behavior.",
      },
      {
        name: "emailDispatcher.smtp.insecureSkipVerify",
        when: { name: "emailDispatcher.driver", value: "smtp" },
        label: "Skip SMTP Certificate Verification",
        type: "switch",
        description: "Unsafe outside trusted development environments.",
      },
      {
        name: "emailDispatcher.webhook.url",
        when: { name: "emailDispatcher.driver", value: "webhook" },
        label: "Email Webhook URL",
        description: "Full HTTP or HTTPS destination for outbound email payloads.",
      },
      {
        name: "emailDispatcher.webhook.signingSecret",
        when: { name: "emailDispatcher.driver", value: "webhook" },
        label: "Email Webhook Signing Secret",
        type: "password",
        configured: "emailDispatcher.webhook.signingSecretConfigured",
        description: "Leave blank to preserve the saved payload-signing secret.",
      },
      {
        name: "emailDispatcher.webhook.timeoutSeconds",
        when: { name: "emailDispatcher.driver", value: "webhook" },
        label: "Email Webhook Timeout (Seconds)",
        type: "number",
        min: 0,
        description: "Use 0 for the backend timeout default.",
      },
    ],
  },
];

export const authSMSSections: AuthSection[] = [
  {
    label: "SMS Provider",
    description:
      "Twilio or webhook delivery for phone verification, sign-in OTP, and password-reset messages.",
    fields: [
      {
        name: "smsDispatcher.driver",
        label: "SMS Provider",
        type: "select",
        options: ["", "twilio", "webhook"],
        description: "Choose a delivery transport, or leave unconfigured.",
      },
      {
        name: "smsDispatcher.twilio.accountSid",
        when: { name: "smsDispatcher.driver", value: "twilio" },
        label: "Twilio Account SID",
        description: "Account identifier from your Twilio project.",
      },
      {
        name: "smsDispatcher.twilio.fromNumber",
        when: { name: "smsDispatcher.driver", value: "twilio" },
        label: "Twilio Sender Number",
        description: "E.164 sender phone number configured in Twilio.",
      },
      {
        name: "smsDispatcher.twilio.authToken",
        when: { name: "smsDispatcher.driver", value: "twilio" },
        label: "Twilio Auth Token",
        type: "password",
        configured: "smsDispatcher.twilio.authTokenConfigured",
        description: "Leave blank to preserve the saved Twilio token.",
      },
      {
        name: "smsDispatcher.webhook.url",
        when: { name: "smsDispatcher.driver", value: "webhook" },
        label: "SMS Webhook URL",
        description: "Full HTTP or HTTPS destination for outbound SMS payloads.",
      },
      {
        name: "smsDispatcher.webhook.signingSecret",
        when: { name: "smsDispatcher.driver", value: "webhook" },
        label: "SMS Webhook Signing Secret",
        type: "password",
        configured: "smsDispatcher.webhook.signingSecretConfigured",
        description: "Leave blank to preserve the saved payload-signing secret.",
      },
      {
        name: "smsDispatcher.webhook.timeoutSeconds",
        when: { name: "smsDispatcher.driver", value: "webhook" },
        label: "SMS Webhook Timeout (Seconds)",
        type: "number",
        min: 0,
        description: "Use 0 for the backend timeout default.",
      },
    ],
  },
];

export const authEmailTemplates: AuthMessageTemplate[] = [
  {
    key: "passwordReset",
    messageKind: "password_reset",
    label: "Password Reset",
    description:
      "Sent when an existing user requests a password reset by email. Password registration and reset must be enabled; email-code sign-in does not need to be enabled.",
  },
  {
    key: "signInOtp",
    messageKind: "sign_in_otp",
    label: "Sign-In Code",
    description:
      "Sent by direct API and hosted email-code sign-in requests. Email OTP and an email provider must be configured. A successful code verification can create a new user.",
  },
  {
    key: "emailVerification",
    messageKind: "email_verification",
    label: "Email Verification",
    description:
      "Sent when your application explicitly requests email verification. Creating an account or signing up does not automatically send this message.",
  },
  {
    key: "suspiciousActivity",
    messageKind: "suspicious_activity",
    label: "Suspicious Activity",
    description:
      "Sent for a new device during API and hosted sign-in when new-device notifications are enabled.",
  },
].map(({ key, messageKind, label, description }): AuthMessageTemplate => ({
  label,
  messageKind,
  description,
  variables: [
    "{{.AppName}}",
    "{{.Recipient}}",
    "{{.To}}",
    ...(key === "suspiciousActivity" ? ["{{.Device}}", "{{.IP}}"] : ["{{.Code}}"]),
  ],
  fields: [
    {
      name: `emailDispatcher.templates.${key}.subject`,
      label: "Subject",
      description:
        "A configured email requires a subject and at least one body; otherwise the backend uses its built-in message.",
    },
    {
      name: `emailDispatcher.templates.${key}.html`,
      label: "HTML Body",
      type: "textarea",
      description: "HTML body template. This field never renders raw HTML in the console.",
    },
    {
      name: `emailDispatcher.templates.${key}.text`,
      label: "Plain-Text Body",
      type: "textarea",
      description: "Plain-text body template.",
    },
  ],
}));

export const authSMSTemplates: AuthMessageTemplate[] = [
  {
    key: "passwordReset",
    messageKind: "password_reset",
    label: "Password Reset",
    description:
      "Sent when an existing user requests a password reset by phone. Password registration and reset must be enabled; SMS-code sign-in does not need to be enabled.",
  },
  {
    key: "signInOtp",
    messageKind: "sign_in_otp",
    label: "Sign-In Code",
    description:
      "Sent by direct API and hosted SMS-code sign-in requests. SMS OTP and an SMS provider must be configured. A successful code verification can create a new user.",
  },
  {
    key: "phoneVerification",
    messageKind: "phone_verification",
    label: "Phone Verification",
    description:
      "Sent when your application explicitly requests phone verification. Creating an account or signing up does not automatically send this message.",
  },
].map(({ key, messageKind, label, description }): AuthMessageTemplate => ({
  label,
  messageKind,
  description,
  variables: ["{{.AppName}}", "{{.Code}}", "{{.Recipient}}", "{{.To}}"],
  fields: [
    {
      name: `smsDispatcher.templates.${key}.text`,
      label: "Message Body",
      type: "textarea",
      description:
        "Leave empty to use the backend's built-in message. SMS contains plain text, not HTML.",
    },
  ],
}));

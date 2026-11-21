import { createClient, useAuth as useAuthService, withServiceAccount } from "layr-client";
const endpoint = process.env.LAYR__SERVER__BASE_URL;
const serviceAccountKey = process.env.LAYR__SERVICE_ACCOUNT_KEY;
if (!endpoint || !serviceAccountKey)
  throw new Error("Run Auth console tests through scripts/check-console-ui.");
export const rootAuth = withServiceAccount(useAuthService(createClient({ endpoint })), {
  serviceAccountKey,
});
export const authEndpoint = endpoint;
export function authTestUser() {
  return {
    email: `auth-ui-${crypto.randomUUID()}@example.com`,
    phone: "",
    password: "Auth-user-test-password-123!",
    role: "authenticated",
    emailVerified: false,
    phoneVerified: false,
    properties: {},
  };
}

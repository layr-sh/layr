import { LayrAPIError } from "layr-client";

export interface ConsoleUser {
  id: string;
  email: string;
  mfa_enabled: boolean;
  is_enabled: boolean;
  last_signed_in_at?: string | null;
  created_at: string;
  last_updated_at: string;
}

export interface ConsoleSession {
  id: string;
  user_id: string;
  ip_address: string;
  user_agent: string;
  expires_at: string;
  created_at: string;
  user: ConsoleUser;
  scopes: string[];
  mfa_required: boolean;
}

export interface ConsoleSignInResponse {
  user: ConsoleUser;
  access_token: string;
  expires_in: number;
  token_type: string;
  mfa_required: boolean;
  mfa_ticket: string;
  factor: string;
}

export type ConsoleSignInResult =
  | { response: ConsoleSignInResponse; error: null }
  | { response: null; error: LayrAPIError };

export type ConsoleSignOutResult =
  | { response: undefined; error: null }
  | { response: null; error: LayrAPIError };

export default defineNuxtPlugin(async ({ $client }) => {
  const state = useState<ConsoleSession | null>("auth", () => null);
  const mfaTicket = useState<string | null>("auth_mfa_ticket", () => null);

  const clear = () => {
    $client.value.clearSession();
    state.value = null;
    mfaTicket.value = null;
  };

  const initialize = async (): Promise<ConsoleSession | null> => {
    try {
      const response = await fetch(`${$client.value.endpoint}/v1/_/console/session`, {
        credentials: "include",
        headers: {
          Accept: "application/json",
          ...($client.value.sessionToken
            ? { Authorization: `Bearer ${$client.value.sessionToken.replace(/^Bearer\s+/i, "")}` }
            : {}),
        },
      });

      if (!response.ok) {
        const error = await LayrAPIError.fromResponse(response);
        console.error(error);
        if (response.status === 401) clear();
        return null;
      }

      const consoleSession: ConsoleSession = await response.json();
      state.value = consoleSession;
      return consoleSession;
    } catch (err) {
      console.error(err);
      return null;
    }
  };

  const signIn = async (email: string, password: string): Promise<ConsoleSignInResult> => {
    try {
      const response = await fetch(`${$client.value.endpoint}/v1/_/console/sign-in`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          email: email.trim(),
          password,
        }),
      });

      if (!response.ok) {
        const error = await LayrAPIError.fromResponse(response);
        console.error(error);
        return {
          response: null,
          error,
        };
      }

      const consoleSignInResponse: ConsoleSignInResponse = await response.json();
      if (consoleSignInResponse.mfa_required) {
        mfaTicket.value = consoleSignInResponse.mfa_ticket;
        return {
          response: consoleSignInResponse,
          error: null,
        };
      }

      if (consoleSignInResponse.access_token) {
        $client.value.setSession(consoleSignInResponse.access_token);
      }
      mfaTicket.value = null;
      await initialize();

      return {
        response: consoleSignInResponse,
        error: null,
      };
    } catch (err) {
      console.error(err);
      return {
        response: null,
        error: new LayrAPIError({
          detail: err instanceof Error ? err.message : "Failed to sign in",
        }),
      };
    }
  };

  const verifyMFA = async (code: string): Promise<ConsoleSignInResult> => {
    if (!mfaTicket.value) {
      return {
        response: null,
        error: new LayrAPIError({
          detail: "MFA challenge ticket is missing or expired. Please sign in again.",
        }),
      };
    }

    const trimmedCode = code.trim();
    if (!trimmedCode) {
      return {
        response: null,
        error: new LayrAPIError({
          detail: "MFA code is required",
        }),
      };
    }

    try {
      const response = await fetch(`${$client.value.endpoint}/v1/_/console/mfa/challenge`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          mfa_ticket: mfaTicket.value,
          code: trimmedCode,
        }),
      });

      if (!response.ok) {
        const error = await LayrAPIError.fromResponse(response);
        console.error(error);
        return {
          response: null,
          error,
        };
      }

      const consoleSignInResponse: ConsoleSignInResponse = await response.json();
      if (consoleSignInResponse.access_token) {
        $client.value.setSession(consoleSignInResponse.access_token);
      }
      mfaTicket.value = null;
      await initialize();

      return {
        response: consoleSignInResponse,
        error: null,
      };
    } catch (err) {
      console.error(err);
      return {
        response: null,
        error: new LayrAPIError({
          detail: err instanceof Error ? err.message : "Failed to verify MFA",
        }),
      };
    }
  };

  const signOut = async (): Promise<ConsoleSignOutResult> => {
    try {
      const response = await fetch(`${$client.value.endpoint}/v1/_/console/sign-out`, {
        method: "POST",
        credentials: "include",
        headers: {
          Accept: "application/json",
          ...($client.value.sessionToken
            ? { Authorization: `Bearer ${$client.value.sessionToken.replace(/^Bearer\s+/i, "")}` }
            : {}),
        },
      });
      if (!response.ok) {
        const error = await LayrAPIError.fromResponse(response);
        console.error(error);
        return {
          response: null,
          error,
        };
      }

      clear();

      return {
        response: undefined,
        error: null,
      };
    } catch (err) {
      console.error(err);
      return {
        response: null,
        error: new LayrAPIError({
          detail: err instanceof Error ? err.message : "Failed to sign out",
        }),
      };
    }
  };

  const setupMFA = async (): Promise<{ secret: string; uri: string }> => {
    const response = await fetch(`${$client.value.endpoint}/v1/_/console/mfa`, {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", ...$client.value.headers },
    });
    if (!response.ok) throw await LayrAPIError.fromResponse(response);
    return response.json();
  };

  const enableMFA = async (code: string) => {
    const response = await fetch(`${$client.value.endpoint}/v1/_/console/mfa/verify`, {
      method: "POST",
      credentials: "include",
      headers: { ...$client.value.headers, "Content-Type": "application/json" },
      body: JSON.stringify({ code: code.trim() }),
    });
    if (!response.ok) throw await LayrAPIError.fromResponse(response);
    await initialize();
  };

  const disableMFA = async () => {
    const response = await fetch(`${$client.value.endpoint}/v1/_/console/mfa`, {
      method: "DELETE",
      credentials: "include",
      headers: { Accept: "application/json", ...$client.value.headers },
    });
    if (!response.ok) throw await LayrAPIError.fromResponse(response);
    await initialize();
  };

  await callOnce(initialize);

  return {
    provide: {
      auth: {
        state,
        mfaTicket,
        initialize,
        refresh: initialize,
        signIn,
        verifyMFA,
        signOut,
        clear,
        setupMFA,
        enableMFA,
        disableMFA,
      },
    },
  };
});

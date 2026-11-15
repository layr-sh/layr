import { describe, it, expect } from "vitest";
import { useAuth } from "../../../src/services/auth";
import { createMockServiceContext } from "./helpers";

describe("Auth Service Unit Tests", () => {
  it("exercises auth base methods", async () => {
    const { client, getLastUrl } = createMockServiceContext((url) => {
      if (url.includes("/userinfo")) {
        return { sub: "u-1", email: "u1@example.com" };
      }
      if (url.includes("/user/properties")) {
        return { properties: { theme: "dark" } };
      }
      if (url.endsWith("/passkeys")) {
        return [];
      }
      if (url.endsWith("/user")) {
        return { user: { id: "u-1", email: "u1@example.com" } };
      }
      return {
        access_token: "tok-123",
        expires_in: 3600,
        refresh_token: "ref-123",
        token_type: "Bearer",
        user: { id: "u-1", email: "u1@example.com" },
      };
    });

    const auth = useAuth(client);

    // root auth
    await auth.sign_in({ email: "u@example.com", password: "Password123!", phone: "" });
    expect(getLastUrl()).toContain("/v1/auth/sign-in");

    await auth.sign_up({
      email: "u@example.com",
      password: "Password123!",
      phone: "",
      properties: null,
    });
    expect(getLastUrl()).toContain("/v1/auth/sign-up");

    await auth.sign_out();
    expect(getLastUrl()).toContain("/v1/auth/sign-out");

    // anonymous
    await auth.anonymous.sign_in();
    expect(getLastUrl()).toContain("/v1/auth/anonymous");

    await auth.anonymous.sign_in({});
    expect(getLastUrl()).toContain("/v1/auth/anonymous");

    // mfa
    await auth.mfa.setup();
    expect(getLastUrl()).toContain("/v1/auth/mfa");

    await auth.mfa.verify({ code: "123456" });
    expect(getLastUrl()).toContain("/v1/auth/mfa/verify");

    await auth.mfa.challenge({ mfaTicket: "ticket-1", code: "123456" });
    expect(getLastUrl()).toContain("/v1/auth/mfa/challenge");

    await auth.mfa.disable();
    expect(getLastUrl()).toContain("/v1/auth/mfa");

    // oidc
    await auth.oidc.token();
    expect(getLastUrl()).toContain("/v1/auth/oauth/token");

    await auth.oidc.token({ grant_type: "authorization_code", code: "code1" });
    expect(getLastUrl()).toContain("/v1/auth/oauth/token");

    await auth.oidc.userinfo();
    expect(getLastUrl()).toContain("/v1/auth/oauth/userinfo");

    // otp
    await auth.otp.send({ purpose: "login", recipient: "u@example.com" });
    expect(getLastUrl()).toContain("/v1/auth/otp");

    await auth.otp.verify({ purpose: "login", recipient: "u@example.com", code: "123456" });
    expect(getLastUrl()).toContain("/v1/auth/otp/verify");

    // passkeys sign_in
    await auth.passkeys.sign_in.begin();
    expect(getLastUrl()).toContain("/v1/auth/passkeys/sign-in");

    await auth.passkeys.sign_in.verify({
      challenge: "ch-1",
      credentialId: "pk-1",
      signature: "sig-1",
    });
    expect(getLastUrl()).toContain("/v1/auth/passkeys/sign-in/verify");

    // passkeys sign_up
    await auth.passkeys.sign_up.begin({ userName: "user1" });
    expect(getLastUrl()).toContain("/v1/auth/passkeys/sign-up");

    await auth.passkeys.sign_up.verify({
      challenge: "ch-1",
      credentialId: "pk-1",
      publicKey: "key-1",
    });
    expect(getLastUrl()).toContain("/v1/auth/passkeys/sign-up/verify");

    // password_reset
    await auth.password_reset.request({ email: "u@example.com" });
    expect(getLastUrl()).toContain("/v1/auth/password-reset/request");

    await auth.password_reset.confirm({
      code: "123456",
      password: "NewPassword123!",
      email: "u@example.com",
    });
    expect(getLastUrl()).toContain("/v1/auth/password-reset/confirm");

    // token
    await auth.token.refresh({ refreshToken: "ref-123" });
    expect(getLastUrl()).toContain("/v1/auth/token/refresh");

    // user
    await auth.user.get();
    expect(getLastUrl()).toContain("/v1/auth/user");

    await auth.user.export();
    expect(getLastUrl()).toContain("/v1/auth/user/export");

    await auth.user.email.update({ email: "new@example.com" });
    expect(getLastUrl()).toContain("/v1/auth/user/email");

    await auth.user.email.verification.request();
    expect(getLastUrl()).toContain("/v1/auth/user/email/verification/request");

    await auth.user.email.verification.request({});
    expect(getLastUrl()).toContain("/v1/auth/user/email/verification/request");

    await auth.user.email.verification.confirm({ code: "123456" });
    expect(getLastUrl()).toContain("/v1/auth/user/email/verification/confirm");

    await auth.user.passkeys.list();
    expect(getLastUrl()).toContain("/v1/auth/user/passkeys");

    await auth.user.passkeys.delete("pk-1");
    expect(getLastUrl()).toContain("/v1/auth/user/passkeys/pk-1");

    await auth.user.password.update({
      currentPassword: "OldPassword123!",
      newPassword: "NewPassword123!",
    });
    expect(getLastUrl()).toContain("/v1/auth/user/password");

    await auth.user.phone.update({ phone: "+1234567890" });
    expect(getLastUrl()).toContain("/v1/auth/user/phone");

    await auth.user.phone.verification.request();
    expect(getLastUrl()).toContain("/v1/auth/user/phone/verification/request");

    await auth.user.phone.verification.request({});
    expect(getLastUrl()).toContain("/v1/auth/user/phone/verification/request");

    await auth.user.phone.verification.confirm({ code: "123456" });
    expect(getLastUrl()).toContain("/v1/auth/user/phone/verification/confirm");

    await auth.user.properties.update({ properties: { theme: "dark" } });
    expect(getLastUrl()).toContain("/v1/auth/user/properties");

    await auth.user.sessions.list();
    expect(getLastUrl()).toContain("/v1/auth/user/sessions");

    await auth.user.sessions.revoke("s-1");
    expect(getLastUrl()).toContain("/v1/auth/user/sessions/s-1");

    await auth.user.sessions.revoke_others();
    expect(getLastUrl()).toContain("/v1/auth/user/sessions/revoke-others");

    await auth.user.delete();
    expect(getLastUrl()).toContain("/v1/auth/user");
  });

  it("exercises auth control plane (.cp) endpoints", async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } =
      createMockServiceContext();
    const auth = elevate(useAuth(client));

    // config
    await auth.cp.config.get();
    expect(getLastUrl()).toContain("/v1/_/auth/config");
    expect(getLastInit()?.headers).toMatchObject({ "Service-Account-Key": serviceAccountKey });

    await auth.cp.config.update({});
    expect(getLastUrl()).toContain("/v1/_/auth/config");

    // users
    await auth.cp.users.create({
      email: "user@example.com",
      password: "Password123!",
      phone: "",
      properties: {},
      role: "authenticated",
    });
    expect(getLastUrl()).toContain("/v1/_/auth/users");

    await auth.cp.users.get("u-1");
    expect(getLastUrl()).toContain("/v1/_/auth/users/u-1");

    await auth.cp.users.list();
    expect(getLastUrl()).toContain("/v1/_/auth/users");

    await auth.cp.users.list({ limit: 10 });
    expect(getLastUrl()).toContain("/v1/_/auth/users");

    await auth.cp.users.lock("u-1", { lockReason: "spam" });
    expect(getLastUrl()).toContain("/v1/_/auth/users/u-1/lock");

    await auth.cp.users.unlock("u-1");
    expect(getLastUrl()).toContain("/v1/_/auth/users/u-1/lock");

    await auth.cp.users.delete("u-1");
    expect(getLastUrl()).toContain("/v1/_/auth/users/u-1");

    // users sessions
    await auth.cp.users.sessions.list("u-1");
    expect(getLastUrl()).toContain("/v1/_/auth/users/u-1/sessions");

    await auth.cp.users.sessions.revoke("u-1");
    expect(getLastUrl()).toContain("/v1/_/auth/users/u-1/sessions/revoke");
  });
});

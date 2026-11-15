import { describe, it, expect } from "vitest";
import { useConsole } from "../../../src/services/console";
import { createMockServiceContext } from "./helpers";

describe("Console Service Unit Tests", () => {
  it("exercises console control plane (.cp) endpoints", async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } =
      createMockServiceContext();
    const consoleSvc = elevate(useConsole(client));

    // config
    await consoleSvc.cp.config.get();
    expect(getLastUrl()).toContain("/v1/_/console/config");
    expect(getLastInit()?.headers).toMatchObject({ "Service-Account-Key": serviceAccountKey });

    await consoleSvc.cp.config.update({});
    expect(getLastUrl()).toContain("/v1/_/console/config");

    // users
    await consoleSvc.cp.users.create({
      email: "admin@example.com",
      password: "Pass",
      name: "Admin",
      role: "admin",
    });
    expect(getLastUrl()).toContain("/v1/_/console/users");

    await consoleSvc.cp.users.get("u-1");
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1");

    await consoleSvc.cp.users.list();
    expect(getLastUrl()).toContain("/v1/_/console/users");

    await consoleSvc.cp.users.update("u-1", { name: "Admin 2" });
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1");

    await consoleSvc.cp.users.reset_password("u-1", { newPassword: "NewPassword123!" });
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1/password");

    await consoleSvc.cp.users.delete("u-1");
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1");

    // sessions
    await consoleSvc.cp.users.sessions.list("u-1");
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1/sessions");

    await consoleSvc.cp.users.sessions.delete("u-1", "s-1");
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1/sessions/s-1");

    await consoleSvc.cp.users.sessions.revoke("u-1");
    expect(getLastUrl()).toContain("/v1/_/console/users/u-1/sessions/revoke");
  });
});

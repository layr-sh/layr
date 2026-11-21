import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UModal } from "#components";
import Users from "../../pages/auth/users.vue";
import { authTestUser, rootAuth } from "./helpers";
import {
  action,
  activeDialog,
  field,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let userIds: string[];
beforeEach(async () => {
  await signInConsoleUser();
  userIds = [];
});
afterEach(async () => {
  wrapper?.unmount();
  for (const id of userIds) await rootAuth.cp.users.delete(id);
  await useNuxtApp().$router.push("/");
  vi.restoreAllMocks();
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
}
function row(identity: string) {
  return pageDOM()
    .findAll("tbody tr")
    .find((item) => item.text().includes(identity))!;
}
async function fixture(emailVerified = false) {
  const user = await rootAuth.cp.users.create({ ...authTestUser(), emailVerified });
  userIds.push(user.id);
  return user;
}
function modal(title: string) {
  return wrapper.findAllComponents(UModal).find((item) => item.props("title") === title)!;
}

describe("Auth user directory interaction and recovery", () => {
  it("validates before provisioning and retains a failed creation form", async () => {
    const existing = await fixture(true);
    ({ wrapper } = await mountPage(Users));
    const state = wrapper.setupState;
    await row(existing.email).find("td:last-child > div").trigger("click");
    expect(state.viewing.value).toBe(false);
    expect(row(existing.email).text()).toContain("Email Verified");
    await state.createUser();
    expect(state.validate().length).toBeGreaterThan(0);
    await action("New User").trigger("click");
    await state.createUser();
    expect(activeDialog()?.textContent).toContain("Enter a valid email");
    await field("Email").setValue(existing.email);
    await field("JWT Role").setValue("member");
    await field("Phone").setValue("+14155552671");
    await field("Email Already Verified").trigger("click");
    await field("Phone Already Verified").trigger("click");
    await state.createUser();
    await settle();
    expect(state.creating.value).toBe(true);
    expect(activeDialog()?.textContent).toContain("Creation Failed");
    expect(state.form.email).toBe(existing.email);
    modal("Create User").vm.$emit("update:open", false);
    await nextTick();
    expect(state.creating.value).toBe(false);
    await action("New User").trigger("click");
    expect(state.form.email).toBe("");
    await action("Cancel", activeDialog()!).trigger("click");
    expect(state.creating.value).toBe(false);
  });

  it("provisions verified phone accounts and displays nullable identity metadata", async () => {
    ({ wrapper } = await mountPage(Users));
    await action("New User").trigger("click");
    const phone = `+1555${Date.now().toString().slice(-7)}`;
    await field("Phone").setValue(phone);
    await field("Phone Already Verified").trigger("click");
    await field("JWT Role").setValue("phone-user");
    await pageDOM().get("#auth-user-create").trigger("submit");
    await settle();
    const user = (await rootAuth.cp.users.list({ search: phone })).data![0]!;
    userIds.push(user.id);
    expect(row(phone).text()).toContain("Phone Verified");
    await menuAction("Details", row(phone).element);
    await settle();
    const state = wrapper.setupState;
    state.authControlPlane.user.value.emailVerifiedAt = new Date();
    state.authControlPlane.user.value.mfaEnabled = true;
    state.authControlPlane.user.value.isAnonymous = true;
    state.authControlPlane.user.value.properties = null;
    await nextTick();
    expect(activeDialog()?.textContent).toContain("Anonymous");
    expect(activeDialog()?.textContent).toContain("Enrolled");
    expect(activeDialog()?.textContent).toContain("{}");
    modal(phone).vm.$emit("update:open", false);
    await nextTick();
    expect(state.viewing.value).toBe(false);
    state.authControlPlane.users.value.data = [
      { ...user, phoneVerifiedAt: undefined, mfaEnabled: true, isAnonymous: true },
    ];
    await nextTick();
    expect(row(phone).text()).toContain("Phone Unverified");
    expect(row(phone).text()).toContain("Anonymous");
    state.authControlPlane.user.value = null;
    state.viewing.value = true;
    await nextTick();
    expect(activeDialog()?.textContent).toContain("User Details");
    await action("Close", activeDialog()!).trigger("click");
  });

  it("supports detail actions and validates explicit future lock times", async () => {
    const user = await fixture();
    ({ wrapper } = await mountPage(Users));
    const state = wrapper.setupState;
    await state.apply();
    await row(user.email).trigger("click");
    await settle();
    await action("Lock User", activeDialog()!).trigger("click");
    expect(state.viewing.value).toBe(false);
    state.lockUntil.value = "invalid-date";
    await state.apply();
    expect(state.lockError.value).toContain("future");
    state.lockUntil.value = "2000-01-01T00:00";
    await state.apply();
    expect(state.lockError.value).toContain("future");
    await field("Lock Until (Local Time)").setValue(
      new Date(Date.now() + 86400000).toISOString().slice(0, 16),
    );
    await state.apply();
    await settle();
    expect((await rootAuth.cp.users.get(user.id)).lockedUntil!.getTime()).toBeGreaterThan(
      Date.now(),
    );
    await row(user.email).trigger("click");
    await settle();
    await action("Unlock User", activeDialog()!).trigger("click");
    await action("Cancel", activeDialog()!).trigger("click");
    await menuAction("Unlock User", row(user.email).element);
    await action("Unlock User", activeDialog()!).trigger("click");
    await settle();
    await row(user.email).trigger("click");
    await settle();
    await pageDOM()
      .findAll("a")
      .find((link) => link.text() === "View Sessions")!
      .trigger("click");
    expect(state.viewing.value).toBe(false);
    await vi.waitFor(() =>
      expect(useNuxtApp().$router.currentRoute.value.query.user_id).toBe(user.id),
    );
  });

  it("keeps stale record actions recoverable and supports confirmation dismissal", async () => {
    const user = await fixture();
    ({ wrapper } = await mountPage(Users));
    const state = wrapper.setupState;
    state.confirm("delete", user);
    await nextTick();
    modal("Delete User?").vm.$emit("update:open", true);
    expect(state.confirmation.value).not.toBeNull();
    await action("Close", activeDialog()!).trigger("click");
    expect(state.confirmation.value).toBeNull();
    state.confirm("delete", user);
    await nextTick();
    await action("Cancel", activeDialog()!).trigger("click");
    state.confirm("delete", user);
    await rootAuth.cp.users.delete(user.id);
    userIds = [];
    await state.apply();
    await settle();
    expect(state.confirmation.value).not.toBeNull();
    expect(activeDialog()?.textContent).toContain("Action Failed");
    await action("Cancel", activeDialog()!).trigger("click");
    await state.details(user.id);
    await settle();
    expect(state.viewing.value).toBe(false);
    expect(pageDOM().text()).toContain("User not found");
  });

  it("filters and paginates without inventing a total, and returns to the previous page after deleting its final row", async () => {
    const role = `page-${crypto.randomUUID()}`;
    const users = await Promise.all(
      Array.from({ length: 21 }, (_, index) =>
        rootAuth.cp.users.create({
          ...authTestUser(),
          password: "",
          role,
          email: `${role}-${index}@example.com`,
        }),
      ),
    );
    userIds = users.map((user) => user.id);
    ({ wrapper } = await mountPage(Users));
    const state = wrapper.setupState;
    await field("Search Email or Phone").setValue(role);
    await field("JWT Role Filter").setValue(role);
    await pageDOM().get("form").trigger("submit");
    await settle();
    expect(state.authControlPlane.users.value.data).toHaveLength(20);
    await action("Next").trigger("click");
    await settle();
    expect(state.authControlPlane.offset.value).toBe(20);
    await action("Previous").trigger("click");
    await settle();
    expect(state.authControlPlane.offset.value).toBe(0);
    await action("Next").trigger("click");
    await settle();
    const last = state.authControlPlane.users.value.data[0];
    await menuAction("Delete User", row(last.email).element);
    await action("Delete User", activeDialog()!).trigger("click");
    await settle();
    userIds = userIds.filter((id) => id !== last.id);
    expect(state.authControlPlane.offset.value).toBe(0);
    await action("Refresh").trigger("click");
    await settle();
    await field("Search Email or Phone").setValue("does-not-match-any-user");
    await pageDOM().get("form").trigger("submit");
    await settle();
    expect(pageDOM().text()).toContain("No Matching Users");
    const remaining = users.find((user) => user.id !== last.id)!;
    state.authControlPlane.users.value = null;
    expect(await state.authControlPlane.deleteUser(remaining.id)).toBe(true);
    userIds = userIds.filter((id) => id !== remaining.id);
  });
});

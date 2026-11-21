import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UModal, USelectMenu } from "#components";
import { createClient, useAuth as useAuthService } from "layr-client";
import Sessions from "../../pages/auth/sessions.vue";
import { authEndpoint, authTestUser, rootAuth } from "./helpers";
import {
  action,
  activeDialog,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let userId: string;
let extraUserIds: string[];
beforeEach(async () => {
  await signInConsoleUser();
  userId = "";
  extraUserIds = [];
});
afterEach(async () => {
  wrapper?.unmount();
  if (userId) await rootAuth.cp.users.delete(userId);
  for (const id of extraUserIds) await rootAuth.cp.users.delete(id);
  vi.restoreAllMocks();
  await useNuxtApp().$router.push("/");
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
}
function modal(title: string) {
  return wrapper.findAllComponents(UModal).find((item) => item.props("title") === title)!;
}
async function fixture() {
  const input = authTestUser();
  const user = await rootAuth.cp.users.create(input);
  userId = user.id;
  const authService = useAuthService(
    createClient({
      endpoint: authEndpoint,
      publishableKey: useNuxtApp().$manifest.state.value.publishableKey,
    }),
  );
  await authService.sign_in({ email: input.email, phone: "", password: input.password });
  return user;
}
describe("Auth per-user session inspection", () => {
  it("validates user lookup and keeps failures and missing selections safe", async () => {
    ({ wrapper } = await mountPage(Sessions));
    const state = wrapper.setupState;
    await state.load();
    expect(state.validate()).toHaveLength(1);
    await state.revoke();
    expect(state.revoking.value).toBe(false);
    await state.chooseUser(undefined);
    await state.chooseUser({ id: crypto.randomUUID(), label: "Removed user" });
    await settle();
    expect(pageDOM().text()).toContain("User not found");
    state.revoking.value = true;
    await nextTick();
    expect(activeDialog()?.textContent).toContain("this user");
    modal("Revoke All Sessions?").vm.$emit("update:open", false);
    await nextTick();
    expect(state.revoking.value).toBe(false);
  });
  it("renders expired, OIDC, and nullable metadata without exposing credentials", async () => {
    const user = await fixture();
    ({ wrapper } = await mountPage(Sessions));
    const state = wrapper.setupState;
    wrapper
      .findComponent(USelectMenu)
      .vm.$emit("update:modelValue", { id: user.id, label: user.email });
    await nextTick();
    await settle();
    const session = state.authControlPlane.sessions.value.data[0];
    session.expiresAt = new Date(0);
    session.ipAddress = "127.0.0.1";
    session.clientId = "oidc-client";
    session.userAgent = "Example Browser";
    await nextTick();
    expect(pageDOM().text()).toContain("Expired");
    expect(pageDOM().text()).toContain("oidc-client");
    const row = pageDOM().findAll("tbody tr")[0]!;
    await row.find("td:last-child > div").trigger("click");
    expect(state.selected.value).toBeNull();
    await menuAction("Details", row.element);
    expect(activeDialog()?.textContent).toContain("Example Browser");
    expect(activeDialog()?.textContent).not.toContain(session.refreshTokenHash);
    modal("Session Details").vm.$emit("update:open", true);
    await nextTick();
    expect(state.selected.value).not.toBeNull();
    await action("Close", activeDialog()!).trigger("click");
    session.clientId = session.ipAddress = session.userAgent = null;
    await nextTick();
    await row.trigger("click");
    expect(activeDialog()?.textContent).toContain("Not Recorded");
    modal("Session Details").vm.$emit("update:open", false);
    await nextTick();
    expect(state.selected.value).toBeNull();
    state.authControlPlane.sessions.value.data = null;
    await nextTick();
    expect(pageDOM().text()).toContain("No Session Records");
  });
  it("preserves a failed bulk revocation and does not expose write actions to readers", async () => {
    const user = await fixture();
    await useNuxtApp().$router.push({ path: "/auth/sessions", query: { user_id: user.id } });
    ({ wrapper } = await mountPage(Sessions));
    const state = wrapper.setupState;
    await action("Revoke All Sessions").trigger("click");
    await signInConsoleUser(["auth:user.read"]);
    await action("Revoke Sessions", activeDialog()!).trigger("click");
    await settle();
    expect(activeDialog()?.textContent).toContain("Revocation Failed");
    expect(state.revoking.value).toBe(true);
    await action("Cancel", activeDialog()!).trigger("click");
    expect(pageDOM().text()).not.toContain("Revoke All Sessions");
    expect((await rootAuth.cp.users.sessions.list(user.id)).data).toHaveLength(1);
  });
  it("searches beyond the first page and automatically loads a selected user's sessions", async () => {
    const user = await fixture();
    for (let index = 0; index < 21; index++) {
      const created = await rootAuth.cp.users.create({ ...authTestUser(), password: "" });
      extraUserIds.push(created.id);
    }
    ({ wrapper } = await mountPage(Sessions));
    const state = wrapper.setupState;
    const picker = wrapper.findComponent(USelectMenu);
    expect(pageDOM().text()).not.toContain("UUID");
    expect(state.userOptions.value.some((item: { id: string }) => item.id === user.id)).toBe(false);
    picker.vm.$emit("update:searchTerm", user.email);
    await vi.waitFor(() =>
      expect(state.userOptions.value.map((item: { id: string }) => item.id)).toContain(user.id),
    );
    await picker.get('button[aria-haspopup="listbox"]').trigger("click");
    await flushPromises();
    expect(pageDOM().text()).toContain(user.email);
    picker.vm.$emit(
      "update:modelValue",
      state.userOptions.value.find((item: { id: string }) => item.id === user.id),
    );
    await settle();
    expect(state.authControlPlane.user.value.id).toBe(user.id);
    expect(state.chosenUser.value.label).toBe(user.email);
    expect(state.authControlPlane.sessions.value.data).toHaveLength(1);
    expect(action("Refresh Sessions").attributes("disabled")).toBeUndefined();
    state.directory.value = null;
    await nextTick();
    expect(state.userOptions.value).toEqual([]);
    expect(state.chosenUser.value.label).toBe(user.email);
    state.directoryError.value = { message: "Directory unavailable" };
    await nextTick();
    expect(pageDOM().text()).toContain("User Search Failed");
    await action("Retry User Search").trigger("click");
    await vi.waitFor(() => expect(state.directoryError.value).toBeFalsy());
    expect(state.userOptions.value.some((item: { id: string }) => item.id === user.id)).toBe(true);
    picker.vm.$emit("update:searchTerm", "no-matching-user@example.invalid");
    await vi.waitFor(() => expect(state.userOptions.value).toEqual([]));
    expect(pageDOM().text()).toContain("No matching users");
  });
  it("labels guest accounts and preserves deep-linked selections", async () => {
    const authService = useAuthService(
      createClient({
        endpoint: authEndpoint,
        publishableKey: useNuxtApp().$manifest.state.value.publishableKey,
      }),
    );
    const result = await authService.anonymous.sign_in({});
    userId = result.user.id;
    await useNuxtApp().$router.push({ path: "/auth/sessions", query: { user_id: userId } });
    ({ wrapper } = await mountPage(Sessions));
    const state = wrapper.setupState;
    expect(state.chosenUser.value).toEqual({ id: userId, label: "Guest account" });
    expect(state.userOptions.value.find((item: { id: string }) => item.id === userId)?.label).toBe(
      "Guest account",
    );
  });
});

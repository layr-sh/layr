import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { nextTick, type Component } from "vue";
import { useNuxtApp } from "#app";
import { UCard, UFormField, USelectMenu, USeparator } from "#components";
import Configuration from "../../pages/auth/configurations.vue";
import HostedSignIn from "../../pages/auth/hosted-sign-in.vue";
import SignInMethods from "../../pages/auth/sign-in-methods.vue";
import Email from "../../pages/auth/email.vue";
import SMS from "../../pages/auth/sms.vue";
import APIs from "../../pages/auth/resource-servers.vue";
import Applications from "../../pages/auth/applications.vue";
import Providers from "../../pages/auth/providers.vue";
import Users from "../../pages/auth/users.vue";
import Security from "../../pages/auth/security.vue";
import {
  action,
  activeDialog,
  field,
  mountPage,
  pageDOM,
  selectOption,
  signInConsoleUser,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(async () => {
  wrapper?.unmount();
  await useNuxtApp().$router.push("/");
});

describe("Auth form information hierarchy", () => {
  it("uses a fixed multi-select for backend-supported MFA risk signals", async () => {
    ({ wrapper } = await mountPage(Security));
    const menu = wrapper.findAllComponents(USelectMenu as Component)[0]!;
    const props = menu.props() as {
      multiple: boolean;
      createItem: boolean;
      searchInput: boolean;
      items: { label: string; value: string }[];
    };
    expect(props.multiple).toBe(true);
    expect(props.createItem).toBe(false);
    expect(props.searchInput).toBe(false);
    expect(props.items).toEqual([
      { label: "New Device", value: "new_device" },
      { label: "New IP Address", value: "new_ip" },
      { label: "Excessive Failed Attempts", value: "excessive_failed_attempts" },
      { label: "Critical Failed Attempts", value: "critical_failed_attempts" },
    ]);
    await field("MFA Risk Triggers").trigger("click");
    await pageDOM()
      .findAll('[role="option"]')
      .find((option) => option.text() === "Critical Failed Attempts")!
      .trigger("click");
    expect(wrapper.setupState.authControlPlane.config.value.mfa.riskTriggers).toContain(
      "critical_failed_attempts",
    );
    await pageDOM().get('[role="listbox"]').trigger("keydown", { key: "Escape" });
    wrapper.setupState.authControlPlane.config.value.mfa.riskTriggers = null;
    await nextTick();
    expect(field("MFA Risk Triggers").text()).toContain("Select risk signals");
  });
  it.each([APIs, Applications, Providers])(
    "places directory search guidance in the input placeholder",
    async (page) => {
      ({ wrapper } = await mountPage(page));
      const search = wrapper
        .findAllComponents(UFormField)
        .find((item) => item.props("label") === "Search")!;
      expect(search.props("description")).toBeUndefined();
      expect(field("Search").attributes("placeholder")).toBe("Search by name or identifier…");
    },
  );
  it("places user search and role filter guidance in placeholders", async () => {
    ({ wrapper } = await mountPage(Users));
    for (const label of ["Search Email or Phone", "JWT Role Filter"]) {
      const search = wrapper
        .findAllComponents(UFormField)
        .find((item) => item.props("label") === label)!;
      expect(search.props("description")).toBeUndefined();
      expect(field(label).attributes("placeholder")).toBeTruthy();
    }
  });
  it.each([Configuration, HostedSignIn])(
    "uses the card heading without repeating a single section heading",
    async (page) => {
      ({ wrapper } = await mountPage(page));
      expect(wrapper.findComponent(UCard).props("title")).toBeTruthy();
      expect(wrapper.findComponent(UCard).props("description")).toBeTruthy();
      expect(wrapper.findAllComponents(USeparator)).toHaveLength(0);
      expect(pageDOM().findAll("label").length).toBeGreaterThan(0);
    },
  );
  it("keeps headings for multiple sign-in method sections without collapsible panels", async () => {
    ({ wrapper } = await mountPage(SignInMethods));
    expect(
      wrapper.findAllComponents(USeparator).map((separator) => separator.props("label")),
    ).toEqual(["Password Authentication", "Passkeys", "Passwordless and Guest Access"]);
    expect(pageDOM().find('[data-slot="trigger"][aria-expanded]').exists()).toBe(false);
  });
  it.each([Email, SMS])(
    "separates provider settings from the message template editor",
    async (page) => {
      ({ wrapper } = await mountPage(page));
      const labels = wrapper
        .findAllComponents(USeparator)
        .map((separator) => separator.props("label"));
      expect(labels).toEqual([
        page === Email ? "Email Provider" : "SMS Provider",
        "Message Templates",
      ]);
      expect(pageDOM().text()).toContain("Password Reset");
      await selectOption("Message", "Sign-In Code");
      expect(pageDOM().text()).toContain("Sign-In Code");
    },
  );
  it("does not add a second heading to a single-section API registration form", async () => {
    ({ wrapper } = await mountPage(APIs));
    await action("New API").trigger("click");
    expect(activeDialog()?.textContent).toContain("New API");
    expect(wrapper.findAllComponents(USeparator)).toHaveLength(0);
    expect(pageDOM().text()).toContain("Audience Identifier");
  });
});

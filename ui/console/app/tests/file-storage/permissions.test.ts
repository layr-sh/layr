import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UModal, UDropdownMenu, UButton, UInputNumber } from "#components";
import { Blob as NodeBlob, File as NodeFile } from "node:buffer";
import { createClient, useFileStorage, withServiceAccount } from "layr-client";
import Layout from "../../pages/file-storage.vue";
import Statistics from "../../pages/file-storage/index.vue";
import Buckets from "../../pages/file-storage/buckets.vue";
import Objects from "../../pages/file-storage/objects.vue";
import Signing from "../../pages/file-storage/signing.vue";
import Configuration from "../../pages/file-storage/configurations.vue";
import {
  action,
  activeDialog,
  field,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  selectOption,
  signInConsoleUser,
} from "../control-plane/mount-page";
const endpoint = process.env.LAYR__SERVER__BASE_URL;
const serviceAccountKey = process.env.LAYR__SERVICE_ACCOUNT_KEY;
if (!endpoint || !serviceAccountKey)
  throw new Error("Run File Storage tests through scripts/check-console-ui.");
const rootStorage = withServiceAccount(
  useFileStorage(createClient({ endpoint, headers: { "Service-Account-Key": serviceAccountKey } })),
  { serviceAccountKey },
);
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
let bucket: Awaited<ReturnType<typeof rootStorage.cp.buckets.create>>;
beforeEach(async () => {
  consoleUser = await signInConsoleUser();
  vi.stubGlobal("Blob", NodeBlob);
  vi.stubGlobal("File", NodeFile);
  bucket = await rootStorage.cp.buckets.create({
    name: `permissions-${crypto.randomUUID()}`,
    backend: "database",
    isPublic: false,
    maxFileSizeBytes: 100,
    allowedMimeTypes: [],
  });
});
afterEach(async () => {
  wrapper?.unmount();
  await rootStorage.cp.buckets.delete(bucket.name);
  await useNuxtApp().$router.push("/");
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
async function settle() {
  await flushPromises();
  await vi.waitFor(
    () => expect(wrapper.setupState.fileStorageControlPlane.busy.value).toBe(false),
    { timeout: 10000 },
  );
}
function modal(title: string) {
  return wrapper.findAllComponents(UModal).find((modal) => modal.props("title") === title)!;
}
function row() {
  return pageDOM()
    .findAll("tbody tr")
    .find((row) => row.text().includes(bucket.name))!;
}

describe("File Storage scopes, validation, and recoverable errors", () => {
  it("handles unavailable bucket defaults and supports bucket-only administrators", async () => {
    ({ wrapper } = await mountPage(Buckets));
    const control = wrapper.setupState.fileStorageControlPlane;
    vi.spyOn(control, "load").mockResolvedValueOnce(false);
    await wrapper.setupState.newBucket();
    expect(wrapper.setupState.editing.value).toBe(false);
    control.config.value = null;
    vi.spyOn(control, "load").mockResolvedValueOnce(true);
    await wrapper.setupState.newBucket();
    expect(wrapper.setupState.form.maxFileSizeBytes).toBe(52428800);
    expect(wrapper.setupState.form.allowedMimeTypes).toEqual([]);
    await action("Cancel", activeDialog()!).trigger("click");
    wrapper.unmount();
    await signInConsoleUser(["storage:bucket.read", "storage:bucket.write"]);
    ({ wrapper } = await mountPage(Buckets));
    await wrapper.setupState.newBucket();
    expect(wrapper.setupState.editing.value).toBe(true);
    expect(wrapper.setupState.fileStorageControlPlane.config.value).toBeNull();
  });
  it("gates navigation and does not load pages without their read scopes", async () => {
    await signInConsoleUser([]);
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.items.value).toEqual([]);
    expect(wrapper.setupState.canView.value).toBe(false);
    for (const page of [Statistics, Buckets, Objects, Configuration]) {
      wrapper.unmount();
      ({ wrapper } = await mountPage(page));
      expect(wrapper.setupState.fileStorageControlPlane.buckets.value).toBeNull();
      expect(wrapper.setupState.fileStorageControlPlane.config.value).toBeNull();
      expect(pageDOM().text()).not.toContain("Request Failed");
    }
    expect(pageDOM().text()).toContain("Configuration Unavailable");
    await wrapper.setupState.save();
    expect(wrapper.setupState.validate()).toEqual([]);
  });

  it("keeps bucket and object metadata read-only without exposing mutation actions", async () => {
    await rootStorage.objects.upload_put(
      bucket.name,
      "readonly.txt",
      new Blob(["read only"], { type: "text/plain" }),
    );
    await signInConsoleUser(["storage:bucket.read"]);
    ({ wrapper } = await mountPage(Buckets));
    await menuAction("Details", row().element);
    await settle();
    expect(field("Bucket Name").attributes("disabled")).toBeDefined();
    expect(pageDOM().text()).not.toContain("Save Bucket");
    expect(pageDOM().text()).not.toContain("New Bucket");
    expect(
      (wrapper.findComponent(UDropdownMenu).props() as { items: { label?: string }[] }).items.map(
        (item) => item.label,
      ),
    ).not.toContain("Delete Bucket");
    modal("Bucket Details").vm.$emit("update:open", true);
    await nextTick();
    expect(wrapper.setupState.editing.value).toBe(true);
    await action("Close", activeDialog()!).trigger("click");
    wrapper.unmount();
    ({ wrapper } = await mountPage(Objects));
    await selectOption("Bucket", bucket.name);
    await settle();
    expect(pageDOM().text()).not.toContain("Upload File");
    expect(
      (wrapper.findComponent(UDropdownMenu).props() as { items: { label?: string }[] }).items.map(
        (item) => item.label,
      ),
    ).toEqual(["Details"]);
    await menuAction("Details");
    wrapper.setupState.selectedObject.value.metadata = null;
    await nextTick();
    modal("Object Details").vm.$emit("update:open", true);
    await nextTick();
    modal("Object Details").vm.$emit("update:open", false);
    await flushPromises();
    expect(activeDialog()).toBeNull();
    await menuAction("Details");
    await nextTick();
    wrapper
      .findAllComponents(UButton)
      .find((button) => button.text().trim() === "Close")!
      .vm.$emit("click");
    await flushPromises();
    expect(wrapper.setupState.selectedObject.value).toBeNull();
  });

  it("handles missing buckets, failed bucket saves and deletion, and masked optional S3 fields", async () => {
    ({ wrapper } = await mountPage(Buckets));
    await wrapper.setupState.deleteBucket();
    await wrapper.setupState.details("missing-bucket");
    expect(pageDOM().text()).toContain("Request Failed");
    expect(wrapper.setupState.editing.value).toBe(false);
    await action("New Bucket").trigger("click");
    await settle();
    await field("Bucket Name").setValue(bucket.name);
    await action("Create Bucket", activeDialog()!).trigger("click");
    await settle();
    expect(activeDialog()!.textContent).toContain("already exists");
    await action("Close", activeDialog()!).trigger("click");
    wrapper.setupState.deleting.value = { ...bucket, name: "missing-bucket" };
    wrapper.setupState.confirmationName.value = "missing-bucket";
    await wrapper.setupState.deleteBucket();
    await nextTick();
    expect(activeDialog()!.textContent).toContain("Deletion Failed");
    modal("Delete File Storage Bucket?").vm.$emit("update:open", true);
    await nextTick();
    expect(wrapper.setupState.deleting.value).not.toBeNull();
    await action("Close", activeDialog()!).trigger("click");
    expect(wrapper.setupState.deleting.value).toBeNull();
    const state = wrapper.setupState.fileStorageControlPlane;
    state.bucket.value = { ...bucket, backend: "s3", backendConfig: null, allowedMimeTypes: null };
    vi.spyOn(state, "getBucket").mockResolvedValueOnce(true);
    await wrapper.setupState.details(bucket.name);
    expect(wrapper.setupState.form.region).toBe("us-east-1");
    expect(wrapper.setupState.form.allowedMimeTypes).toEqual([]);
    await field("S3 Endpoint").setValue("https://s3.example.com");
    await field("S3 Bucket").setValue("remote-bucket");
    await field("S3 Region").setValue("eu-west-1");
    await field("S3 Access Key ID").setValue("access-key");
    await field("S3 Secret Access Key").setValue("replacement-secret");
    await field("Maximum File Size (Bytes)").setValue("200");
    await field("Maximum File Size (Bytes)").trigger("blur");
    await field("Allowed MIME Types").setValue("text/plain");
    await field("Allowed MIME Types").trigger("keydown", { key: "Enter" });
    expect(wrapper.setupState.validate()).toEqual([]);
    await action("Cancel", activeDialog()!).trigger("click");
    await action("New Bucket").trigger("click");
    await settle();
    await selectOption("File Storage Backend", "S3");
    expect(activeDialog()!.textContent).toContain("server encrypts this secret");
    await action("Cancel", activeDialog()!).trigger("click");
    state.buckets.value = null;
    state.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Buckets");
    state.busy.value = false;
    await nextTick();
    expect(pageDOM().text()).toContain("No File Storage Buckets");
  });

  it("validates uploads before review, enforces bucket limits, and preserves failed confirmations", async () => {
    ({ wrapper } = await mountPage(Objects));
    const state = wrapper.setupState;
    await state.upload();
    await state.deleteObject();
    state.reviewUpload();
    expect(state.publicUrl.value).toContain("/objects//");
    expect(state.rows.value).toEqual([]);
    await selectOption("Bucket", bucket.name);
    await settle();
    await action("Upload File").trigger("click");
    state.reviewUpload();
    await action("Review Upload", activeDialog()!).trigger("click");
    expect(activeDialog()!.textContent).toContain("Choose a file to upload");
    state.selectFile(undefined);
    state.selectFile(new File(["too large"], "file.txt"));
    state.fileStorageControlPlane.bucket.value.maxFileSizeBytes = 3;
    expect(state.validateUpload()).toContainEqual(
      expect.objectContaining({ name: "file", message: expect.stringContaining("exceeds") }),
    );
    state.uploadForm.contentType = "invalid";
    expect(state.validateUpload()).toContainEqual(expect.objectContaining({ name: "contentType" }));
    state.uploadForm.key = "../bad";
    await state.upload();
    state.uploadForm.key = "valid.txt";
    state.uploadForm.contentType = "text/plain";
    state.fileStorageControlPlane.bucket.value.allowedMimeTypes = ["image/png"];
    expect(state.validateUpload()).toContainEqual(
      expect.objectContaining({ message: "This MIME type is not allowed by the bucket." }),
    );
    state.fileStorageControlPlane.bucket.value.allowedMimeTypes = ["*/*"];
    state.fileStorageControlPlane.bucket.value.maxFileSizeBytes = 0;
    expect(state.validateUpload()).toEqual([]);
    state.fileStorageControlPlane.bucket.value.allowedMimeTypes = ["TEXT/PLAIN"];
    expect(state.validateUpload()).toEqual([]);
    state.fileStorageControlPlane.bucket.value.allowedMimeTypes = null;
    await field("Object Key").setValue("valid.txt");
    await field("Content Type").setValue("text/plain");
    await action("Review Upload", activeDialog()!).trigger("click");
    await action("Cancel", activeDialog()!).trigger("click");
    state.fileStorageControlPlane.bucket.value = null;
    state.validateUpload();
    await state.upload();
    await state.chooseBucket(bucket.name);
    await action("Upload File").trigger("click");
    state.selectFile(new File(["hello"], "valid.txt", { type: "text/plain" }));
    state.uploadForm.key = "valid.txt";
    await rootStorage.cp.buckets.update(bucket.name, { allowedMimeTypes: ["image/png"] });
    await action("Review Upload", activeDialog()!).trigger("click");
    await action("Upload Object", activeDialog()!).trigger("click");
    await settle();
    expect(activeDialog()!.textContent).toContain("Upload Failed");
    await action("Close", activeDialog()!).trigger("click");
    await action("Cancel", activeDialog()!).trigger("click");
    await action("Upload File").trigger("click");
    await action("Close", activeDialog()!).trigger("click");
    expect(state.uploading.value).toBe(false);
    state.deleting.value = { objectKey: "missing.txt" };
    await state.deleteObject();
    await nextTick();
    expect(activeDialog()!.textContent).toContain("Deletion Failed");
    modal("Delete File Storage Object?").vm.$emit("update:open", true);
    await nextTick();
    await action("Close", activeDialog()!).trigger("click");
    expect(state.deleting.value).toBeNull();
    await state.chooseBucket("missing-bucket");
    expect(pageDOM().text()).toContain("Request Failed");
  });

  it("supports read-only signing without bucket-list access and rejects unauthorized operations", async () => {
    await signInConsoleUser(["storage:object.read"]);
    ({ wrapper } = await mountPage(Signing));
    await wrapper.setupState.sign();
    expect(pageDOM().text()).toContain("Enter the name of an existing bucket");
    expect(wrapper.setupState.signedUrl.value).toBe("");
    await field("Bucket Name").setValue(bucket.name);
    await field("Object Key").setValue("known.txt");
    for (const lifetime of [-1, 1.5, 604801]) {
      wrapper.setupState.form.expiresInSeconds = lifetime;
      expect(wrapper.setupState.validate()).toContainEqual(
        expect.objectContaining({ name: "expiresInSeconds" }),
      );
    }
    wrapper.setupState.form.expiresInSeconds = 3600;
    wrapper.setupState.form.operation = "delete";
    expect(wrapper.setupState.validate()).toContainEqual(
      expect.objectContaining({ name: "operation" }),
    );
    wrapper.setupState.form.operation = "write";
    expect(wrapper.setupState.validate()).toContainEqual(
      expect.objectContaining({ name: "operation" }),
    );
    wrapper.setupState.form.operation = "read";
    await field("URL Lifetime (Seconds)").setValue("90");
    await field("URL Lifetime (Seconds)").trigger("blur");
    wrapper.findComponent(UInputNumber).vm.$emit("update:modelValue", 90);
    await nextTick();
    expect(wrapper.setupState.form.expiresInSeconds).toBe(90);
    await action("Sign URL").trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("Signed File Storage URL");
    await field("Bucket Name").setValue("missing-bucket");
    await action("Sign URL").trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("Request Failed");
    wrapper.unmount();
    await signInConsoleUser([]);
    ({ wrapper } = await mountPage(Signing));
    expect(pageDOM().text()).not.toContain("Sign URL");
  });

  it("validates configuration, supports read-only access, and renders missing or nullable configuration states", async () => {
    ({ wrapper } = await mountPage(Configuration));
    const state = wrapper.setupState;
    state.fileStorageControlPlane.error.value = "Configuration request failed";
    await nextTick();
    expect(pageDOM().text()).toContain("Configuration request failed");
    state.fileStorageControlPlane.error.value = "";
    const config = state.fileStorageControlPlane.config.value;
    for (const key of ["defaultMaxFileSizeBytes", "chunkSizeBytes", "presignTokenExpirySeconds"])
      config[key] = 0;
    state.allowedTypes.value = ["invalid"];
    await state.save();
    expect(state.validate()).toHaveLength(4);
    await field("Default Maximum File Size (Bytes)").setValue("2048");
    await field("Default Maximum File Size (Bytes)").trigger("blur");
    await field("Database Chunk Size (Bytes)").setValue("1024");
    await field("Database Chunk Size (Bytes)").trigger("blur");
    await field("Default URL Lifetime (Seconds)").setValue("60");
    await field("Default URL Lifetime (Seconds)").trigger("blur");
    state.allowedTypes.value = [];
    await field("Default Allowed MIME Types").setValue("text/plain");
    await field("Default Allowed MIME Types").trigger("keydown", { key: "Enter" });
    expect(state.validate()).toEqual([]);
    state.fileStorageControlPlane.config.value = null;
    expect(state.validate()).toEqual([]);
    await state.save();
    state.fileStorageControlPlane.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading File Storage Configuration");
    state.fileStorageControlPlane.busy.value = false;
    wrapper.unmount();
    await signInConsoleUser(["storage:config.read"]);
    ({ wrapper } = await mountPage(Configuration));
    expect(field("Default Maximum File Size (Bytes)").attributes("disabled")).toBeDefined();
    expect(pageDOM().text()).not.toContain("Save File Storage Config");
    const control = wrapper.setupState.fileStorageControlPlane;
    vi.spyOn(control, "load").mockResolvedValueOnce(false);
    await wrapper.setupState.load();
    control.config.value.defaultAllowedMimeTypes = null;
    vi.spyOn(control, "load").mockResolvedValueOnce(true);
    await wrapper.setupState.load();
    expect(wrapper.setupState.allowedTypes.value).toEqual([]);
  });

  it("keeps deletion safe when the current object list is unavailable", async () => {
    await rootStorage.objects.upload_put(
      bucket.name,
      "unlisted.txt",
      new Blob(["data"], { type: "text/plain" }),
    );
    ({ wrapper } = await mountPage(Objects));
    const control = wrapper.setupState.fileStorageControlPlane;
    control.objects.value = null;
    expect(await control.deleteObject(bucket.name, "unlisted.txt")).toBe(true);
    expect(control.objects.value.data).toEqual([]);
    expect(control.objects.value.total || 0).toBe(0);
  });

  it("handles busy guards, non-Error failures, denied scopes, and expired Console sessions", async () => {
    ({ wrapper } = await mountPage(Buckets));
    const state = wrapper.setupState.fileStorageControlPlane;
    state.busy.value = true;
    expect(await state.run(async () => {})).toBe(false);
    state.busy.value = false;
    await state.run(async () => {
      throw "Rejected";
    });
    expect(state.error.value).toBe("The request failed. Please try again.");
    await state.run(async () => {
      throw new Error("Connection failed");
    });
    expect(state.error.value).toBe("Connection failed");
    await signInConsoleUser(["storage:bucket.read"]);
    expect(await state.load("configuration")).toBe(false);
    expect(useNuxtApp().$auth.state.value).not.toBeNull();
    const current = useNuxtApp().$auth.state.value!;
    await consoleUser.serviceAccount.console.users.sessions.delete(current.user.id, current.id);
    expect(await state.load("buckets")).toBe(false);
    expect(useNuxtApp().$auth.state.value).toBeNull();
    expect(useNuxtApp().$router.currentRoute.value.path).toBe("/sign-in");
  });
});

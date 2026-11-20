import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { Blob as NodeBlob, File as NodeFile } from "node:buffer";
import { useNuxtApp } from "#app";
import { UCard, UFormField } from "#components";
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
  {
    serviceAccountKey,
  },
);
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let names: string[];
beforeEach(async () => {
  await signInConsoleUser();
  vi.stubGlobal("Blob", NodeBlob);
  vi.stubGlobal("File", NodeFile);
  names = [];
});
afterEach(async () => {
  wrapper?.unmount();
  for (const name of names)
    if ((await rootStorage.cp.buckets.list()).data?.some((bucket) => bucket.name === name))
      await rootStorage.cp.buckets.delete(name);
  await useNuxtApp().$router.push("/");
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
async function createBucket(isPublic = false) {
  const name = `console-storage-${crypto.randomUUID()}`;
  names.push(name);
  return rootStorage.cp.buckets.create({
    name,
    backend: "database",
    isPublic,
    maxFileSizeBytes: 1024 * 1024,
    allowedMimeTypes: [],
  });
}
async function settle() {
  await flushPromises();
  await vi.waitFor(
    () => expect(wrapper.setupState.fileStorageControlPlane.busy.value).toBe(false),
    {
      timeout: 10000,
    },
  );
}
function row(text: string) {
  return pageDOM()
    .findAll("tbody tr")
    .find((row) => row.text().includes(text))!;
}
async function chooseFile(name: string, body: string) {
  const file = new File([body], name, { type: "text/plain" });
  const input = pageDOM().find('input[type="file"]');
  Object.defineProperty(input.element, "files", { value: [file], configurable: true });
  await input.trigger("change");
  await flushPromises();
  return file;
}

describe("File Storage service integration", () => {
  it("uses ordered scope-aware navigation and standard card properties", async () => {
    ({ wrapper } = await mountPage(Layout));
    expect(
      pageDOM()
        .findAll('nav[aria-label="File Storage Service Settings"] a')
        .map((link) => link.text()),
    ).toEqual(["Statistics", "Buckets", "Objects", "URL Signing", "Configurations"]);
    const router = useNuxtApp().$router;
    for (const path of [
      "/file-storage",
      "/file-storage/buckets",
      "/file-storage/objects",
      "/file-storage/signing",
      "/file-storage/configurations",
    ]) {
      await router.push(path);
      expect(wrapper.setupState.canView.value).toBe(true);
      expect(router.resolve(path).matched.length).toBeGreaterThan(0);
    }
    await router.push("/");
    for (const component of [Statistics, Buckets, Objects, Signing, Configuration]) {
      wrapper.unmount();
      ({ wrapper } = await mountPage(component));
      for (const card of wrapper.findAllComponents(UCard)) {
        expect(card.props("title")).toBeTruthy();
        expect(card.props("description")).toBeTruthy();
      }
    }
  });

  it("reports tracked bucket and object usage from the statistics API", async () => {
    const bucket = await createBucket(true);
    await rootStorage.objects.upload_put(
      bucket.name,
      "stats.txt",
      new Blob(["usage"], { type: "text/plain" }),
    );
    const s3Name = `stats-s3-${crypto.randomUUID()}`;
    names.push(s3Name);
    await rootStorage.cp.buckets.create({
      name: s3Name,
      backend: "s3",
      isPublic: false,
      maxFileSizeBytes: 1024,
      allowedMimeTypes: [],
    });
    ({ wrapper } = await mountPage(Statistics));
    const total = (await rootStorage.cp.buckets.list()).data!.length;
    expect(wrapper.setupState.summary.value[0].value).toBe(total.toLocaleString());
    expect(pageDOM().text()).toContain("Public Buckets");
    expect(pageDOM().text()).toContain("Tracked Objects");
    expect(pageDOM().text()).toContain("text/plain");
    expect(pageDOM().text()).toContain("untracked upstream files");
    const usage = wrapper.setupState.fileStorageControlPlane.stats.value;
    expect(usage.buckets.find((item: { name: string }) => item.name === bucket.name)).toMatchObject(
      { objectCount: 1, sizeBytes: 5 },
    );
    expect(usage.totalObjects).toBeGreaterThanOrEqual(1);
    usage.lastUpdatedAt = null;
    usage.buckets = null;
    usage.contentTypes = null;
    await nextTick();
    expect(pageDOM().text()).toContain("No objects uploaded");
    await action("Refresh").trigger("click");
    await settle();
    wrapper.setupState.fileStorageControlPlane.error.value = "Bucket statistics request failed";
    wrapper.setupState.fileStorageControlPlane.stats.value = null;
    expect(wrapper.setupState.summary.value).toEqual([]);
    await nextTick();
    expect(pageDOM().text()).toContain("Bucket statistics request failed");
    expect(pageDOM().text()).toContain("Statistics Unavailable");
    wrapper.setupState.fileStorageControlPlane.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading File Storage Statistics");
    wrapper.setupState.fileStorageControlPlane.busy.value = false;
  });

  it("creates, edits, and deliberately deletes buckets through their row actions", async () => {
    ({ wrapper } = await mountPage(Buckets));
    await action("New Bucket").trigger("click");
    await settle();
    await action("Cancel", activeDialog()!).trigger("click");
    await action("New Bucket").trigger("click");
    await settle();
    await wrapper.setupState.save();
    expect(activeDialog()!.textContent).toContain("Use 3–63 lowercase");
    const name = `console-ui-${crypto.randomUUID()}`;
    names.push(name);
    await field("Bucket Name").setValue(name);
    await field("Public Bucket").trigger("click");
    await action("Create Bucket", activeDialog()!).trigger("click");
    await settle();
    expect(activeDialog()).toBeNull();
    expect((await rootStorage.cp.buckets.get(name)).isPublic).toBe(true);
    expect(row(name).findAll("td").at(-1)!.find('button[aria-haspopup="menu"]').exists()).toBe(
      true,
    );
    await row(name).trigger("click");
    await settle();
    expect(activeDialog()!.textContent).toContain("Bucket Metadata");
    expect(field("Bucket Name").attributes("disabled")).toBeDefined();
    expect(field("File Storage Backend").attributes("disabled")).toBeDefined();
    await field("Public Bucket").trigger("click");
    await action("Save Bucket", activeDialog()!).trigger("click");
    await settle();
    expect((await rootStorage.cp.buckets.get(name)).isPublic).toBe(false);
    await menuAction("Details", row(name).element);
    await settle();
    await action("Close", activeDialog()!).trigger("click");
    await menuAction("Delete Bucket", row(name).element);
    expect(action("Delete Bucket", activeDialog()!).attributes("disabled")).toBeDefined();
    await wrapper.setupState.deleteBucket();
    await field("Confirm Bucket Name").setValue(name);
    await action("Cancel", activeDialog()!).trigger("click");
    await menuAction("Delete Bucket", row(name).element);
    await field("Confirm Bucket Name").setValue(name);
    await action("Delete Bucket", activeDialog()!).trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("File Storage Bucket Deleted");
  });

  it("edits S3 settings without showing or overwriting stored secrets", async () => {
    const name = `s3-${crypto.randomUUID()}`;
    names.push(name);
    await rootStorage.cp.buckets.create({
      name,
      backend: "s3",
      isPublic: false,
      maxFileSizeBytes: 2048,
      allowedMimeTypes: ["image/png"],
      backendConfig: {
        endpoint: "https://s3.example.com",
        region: "us-east-1",
        bucket: "upstream-assets",
        access_key_id: "test-access",
        secret_access_key: "private-upstream-secret",
      },
    });
    ({ wrapper } = await mountPage(Buckets));
    expect(pageDOM().text()).not.toContain("private-upstream-secret");
    await menuAction("Details", row(name).element);
    await settle();
    for (const label of ["S3 Bucket", "S3 Region"])
      expect(
        wrapper
          .findAllComponents(UFormField)
          .find((field) => field.props("label") === label)!
          .props("description"),
      ).toBeTruthy();
    expect(field("S3 Secret Access Key").element).toHaveProperty("value", "");
    expect(field("S3 Endpoint").element).toHaveProperty("value", "https://s3.example.com");
    await field("S3 Region").setValue("eu-west-1");
    await action("Save Bucket", activeDialog()!).trigger("click");
    await settle();
    expect((await rootStorage.cp.buckets.get(name)).backendConfig).toMatchObject({
      region: "eu-west-1",
      secret_access_key: "********",
    });
  });

  it("uploads binary files to private buckets, downloads them, signs links, and deletes objects", async () => {
    const bucket = await createBucket();
    const router = useNuxtApp().$router;
    await router.push({ path: "/file-storage/objects", query: { bucket: bucket.name } });
    ({ wrapper } = await mountPage(Objects));
    expect(pageDOM().text()).toContain("No Stored Objects");
    await action("Upload File").trigger("click");
    await chooseFile("hello.txt", "hello from storage");
    await field("Object Key").setValue("docs/hello.txt");
    await action("Review Upload", activeDialog()!).trigger("click");
    expect(activeDialog()!.textContent).toContain("Confirm Object Upload?");
    await action("Upload Object", activeDialog()!).trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("File Storage Object Uploaded");
    expect(activeDialog()).toBeNull();
    const object = (await rootStorage.cp.buckets.list_objects(bucket.name)).data![0]!;
    expect(object.objectKey).toBe("docs/hello.txt");
    expect(object.sizeBytes).toBe(18);
    expect(await (await rootStorage.objects.download(bucket.name, object.objectKey)).text()).toBe(
      "hello from storage",
    );
    await row(object.objectKey).trigger("click");
    expect(activeDialog()!.textContent).toContain(object.checksumSha256);
    expect(activeDialog()!.textContent).not.toContain("Public URL");
    await action("Close", activeDialog()!).trigger("click");
    await field("Filter Current Page").setValue("does-not-exist");
    expect(pageDOM().text()).toContain("No Matching Objects");
    await field("Filter Current Page").setValue("");
    const downloaded: Blob[] = [];
    vi.spyOn(URL, "createObjectURL").mockImplementation((blob) => {
      downloaded.push(blob as Blob);
      return "blob:storage-download";
    });
    const revoke = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    await menuAction("Download", row(object.objectKey).element);
    await settle();
    expect(await downloaded[0]!.text()).toBe("hello from storage");
    await vi.waitFor(() => expect(revoke).toHaveBeenCalledWith("blob:storage-download"));
    await rootStorage.cp.buckets.update(bucket.name, { isPublic: true });
    await wrapper.setupState.chooseBucket(bucket.name);
    await row(object.objectKey).trigger("click");
    expect(pageDOM().find('[aria-label="Public Object URL"]').element).toHaveProperty(
      "value",
      `${endpoint}/v1/file-storage/objects/${bucket.name}/docs/hello.txt`,
    );
    await action("Close", activeDialog()!).trigger("click");
    await menuAction("Delete Object", row(object.objectKey).element);
    expect(activeDialog()!.querySelector('[data-slot="body"]')).toBeNull();
    await action("Cancel", activeDialog()!).trigger("click");
    await menuAction("Delete Object", row(object.objectKey).element);
    await action("Delete Object", activeDialog()!).trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("File Storage Object Deleted");
    expect(pageDOM().text()).toContain("No Stored Objects");
    await wrapper.setupState.chooseBucket("");
    expect(pageDOM().text()).toContain("Choose a Bucket");
  });

  it("paginates objects and moves back when deleting the last object on a page", async () => {
    const bucket = await createBucket();
    for (let index = 0; index < 21; index++)
      await rootStorage.objects.upload_put(
        bucket.name,
        `file-${String(index).padStart(2, "0")}.txt`,
        new Blob(["data"], { type: "text/plain" }),
        { contentType: "text/plain" },
      );
    ({ wrapper } = await mountPage(Objects));
    await selectOption("Bucket", bucket.name);
    await settle();
    await action("Next").trigger("click");
    await settle();
    expect(wrapper.setupState.fileStorageControlPlane.offset.value).toBe(20);
    await action("Previous").trigger("click");
    await settle();
    await action("Refresh").trigger("click");
    await settle();
    await action("Next").trigger("click");
    await settle();
    await menuAction("Delete Object", row("file-20.txt").element);
    await action("Delete Object", activeDialog()!).trigger("click");
    await settle();
    expect(wrapper.setupState.fileStorageControlPlane.offset.value).toBe(0);
    expect(wrapper.setupState.fileStorageControlPlane.objects.value.total).toBe(20);
  });

  it("signs usable read and write URLs and clears stale results when the form changes", async () => {
    const bucket = await createBucket();
    await rootStorage.objects.upload_put(
      bucket.name,
      "reports/signed #?.txt",
      new Blob(["signed content"], { type: "text/plain" }),
      {
        contentType: "text/plain",
      },
    );
    await useNuxtApp().$router.push({
      path: "/file-storage/signing",
      query: { bucket: bucket.name, key: "reports/signed #?.txt" },
    });
    ({ wrapper } = await mountPage(Signing));
    for (const label of ["Operation", "URL Lifetime (Seconds)"])
      expect(
        wrapper
          .findAllComponents(UFormField)
          .find((field) => field.props("label") === label)!
          .props("description"),
      ).toBeTruthy();
    await action("Sign URL").trigger("click");
    await settle();
    const url = (field("Signed URL").element as HTMLTextAreaElement).value;
    expect(new URL(url).searchParams.get("op")).toBe("read");
    const response = await fetch(url);
    expect(response.status).toBe(200);
    expect(await response.text()).toBe("signed content");
    await selectOption("Operation", "Write (PUT)");
    expect(pageDOM().text()).toContain("Write Access Can Replace Files");
    expect(pageDOM().text()).not.toContain("Signed File Storage URL");
    await action("Sign URL").trigger("click");
    await settle();
    const writeUrl = (field("Signed URL").element as HTMLTextAreaElement).value;
    expect(
      (
        await fetch(writeUrl, {
          method: "PUT",
          body: "updated",
          headers: { "Content-Type": "text/plain" },
        })
      ).status,
    ).toBe(201);
    expect(
      await (await rootStorage.objects.download(bucket.name, "reports/signed #?.txt")).text(),
    ).toBe("updated");
  });

  it("saves runtime defaults without a service-level enable switch", async () => {
    const original = await rootStorage.cp.config.get();
    try {
      ({ wrapper } = await mountPage(Configuration));
      expect(pageDOM().text()).toContain("existing buckets keep their policies");
      expect(pageDOM().text()).not.toContain("Enable Object Access");
      await field("Default Maximum File Size (Bytes)").setValue("2048");
      await field("Default Maximum File Size (Bytes)").trigger("blur");
      await action("Save File Storage Config").trigger("click");
      await settle();
      expect(await rootStorage.cp.config.get()).toMatchObject({ defaultMaxFileSizeBytes: 2048 });
      expect(await rootStorage.cp.config.get()).not.toHaveProperty("enabled");
      expect(pageDOM().text()).toContain("File Storage Configuration Saved");
    } finally {
      await rootStorage.cp.config.update(original);
    }
  });
});

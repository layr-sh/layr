import { describe, it, expect } from "vitest";
import { useFileStorage } from "../../../src/services/file-storage";
import { createMockServiceContext } from "./helpers";

describe("FileStorage Service Unit Tests", () => {
  it("exercises file storage base endpoints", async () => {
    const { client, getLastUrl, getLastInit } = createMockServiceContext({
      bucket: "avatars",
      key: "u1.png",
      size: 1024,
    });
    const storage = useFileStorage(client);

    const bucketClient = storage.from("avatars");
    expect(bucketClient.bucket).toBe("avatars");

    await storage.objects.download("avatars", "u1.png");
    expect(getLastUrl()).toContain("/v1/file-storage/objects/avatars/u1.png");

    await storage.objects.head("avatars", "u1.png");
    expect(getLastUrl()).toContain("/v1/file-storage/objects/avatars/u1.png");

    await storage.objects.upload_post("avatars", "u1.png", "binary-content");
    expect(getLastUrl()).toContain("/v1/file-storage/objects/avatars/u1.png");

    await storage.objects.upload_put("avatars", "u1.png", "binary-content", {
      contentType: "image/png",
    });
    expect(getLastUrl()).toContain("/v1/file-storage/objects/avatars/u1.png");
    expect(getLastInit()?.headers).toMatchObject({ "Content-Type": "image/png" });

    // upload_put with Blob
    const blob = new Blob(["blob-content"], { type: "text/plain" });
    await storage.objects.upload_put("avatars", "u1.txt", blob);
    expect(getLastInit()?.headers).toMatchObject({ "Content-Type": "text/plain" });

    // upload_put without content type options
    await storage.objects.upload_put("avatars", "raw.bin", "content");
    expect(getLastUrl()).toContain("/v1/file-storage/objects/avatars/raw.bin");

    await storage.objects.delete("avatars", "u1.png");
    expect(getLastUrl()).toContain("/v1/file-storage/objects/avatars/u1.png");

    await storage.url.presign({
      bucket: "avatars",
      key: "u1.png",
      operation: "read",
      expiresInSeconds: 3600,
    });
    expect(getLastUrl()).toContain("/v1/file-storage/url/presign");
  });

  it("exercises file storage control plane (.cp) endpoints", async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } =
      createMockServiceContext();
    const storage = elevate(useFileStorage(client));

    // buckets
    await storage.cp.buckets.create({ name: "docs", isPublic: false });
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets");
    expect(getLastInit()?.headers).toMatchObject({ "Service-Account-Key": serviceAccountKey });

    await storage.cp.buckets.get("docs");
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets/docs");

    await storage.cp.buckets.list();
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets");

    await storage.cp.buckets.list_objects("docs");
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets/docs/objects");

    await storage.cp.buckets.list_objects("docs", { limit: 50 });
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets/docs/objects");

    await storage.cp.buckets.update("docs", { isPublic: true });
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets/docs");

    await storage.cp.buckets.delete("docs");
    expect(getLastUrl()).toContain("/v1/_/file-storage/buckets/docs");

    await storage.cp.stats.get();
    expect(getLastUrl()).toContain("/v1/_/file-storage/stats");
    expect(getLastInit()?.headers).toMatchObject({ "Service-Account-Key": serviceAccountKey });

    // config
    await storage.cp.config.get();
    expect(getLastUrl()).toContain("/v1/_/file-storage/config");

    await storage.cp.config.update({});
    expect(getLastUrl()).toContain("/v1/_/file-storage/config");
  });
});

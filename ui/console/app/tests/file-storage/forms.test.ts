import { describe, expect, it } from "vitest";
import {
  resolveFileStorageBucket,
  fileStorageBytes,
  fileStorageMimeTypes,
  validFileStorageKey,
  validFileStorageMimeTypes,
  type FileStorageBucketForm,
} from "../../utils/file-storage-forms";
const form = (): FileStorageBucketForm => ({
  name: "test-assets",
  backend: "database",
  isPublic: false,
  maxFileSizeBytes: 1024,
  allowedMimeTypes: [],
  endpoint: "",
  region: "us-east-1",
  upstreamBucket: "",
  accessKeyId: "",
  secretAccessKey: "",
});
describe("File Storage form resolution", () => {
  it.each([
    [0, "0 B"],
    [1024, "1 KiB"],
    [1048576, "1 MiB"],
    [1073741824, "1 GiB"],
    [1099511627776, "1 TiB"],
  ])("formats %s bytes as %s", (size, expected) =>
    expect(fileStorageBytes(Number(size))).toBe(expected),
  );
  it.each(["hello.txt", "folder/report.pdf", "spaces and #symbols?.txt", "图片.png"])(
    "accepts object key %s",
    (key) => expect(validFileStorageKey(key)).toBe(true),
  );
  it.each(["", "/leading", "trailing/", "a//b", "a/../b", "./a", "a\\b", "bad\u0000key"])(
    "rejects unsafe object key %s",
    (key) => expect(validFileStorageKey(key)).toBe(false),
  );
  it("normalizes MIME allowlists with type wildcards", () => {
    expect(fileStorageMimeTypes([" IMAGE/PNG ", "image/png", "", "text/plain"])).toEqual([
      "image/png",
      "text/plain",
    ]);
    expect(validFileStorageMimeTypes(["image/png", "application/vnd.api+json", "*/*"])).toBe(true);
    expect(validFileStorageMimeTypes(["image/*"])).toBe(true);
    expect(validFileStorageMimeTypes(["*/png", "image/p*"])).toBe(false);
    expect(validFileStorageMimeTypes(["invalid"])).toBe(false);
  });
  it("resolves database settings and rejects invalid bucket fields", () => {
    expect(resolveFileStorageBucket(form(), true)).toEqual({
      errors: [],
      input: {
        name: "test-assets",
        backend: "database",
        isPublic: false,
        maxFileSizeBytes: 1024,
        allowedMimeTypes: [],
      },
    });
    const invalid = {
      ...form(),
      name: "BAD",
      maxFileSizeBytes: 0,
      backend: "disk",
      allowedMimeTypes: ["invalid"],
    };
    expect(resolveFileStorageBucket(invalid, true).errors.map((error) => error.name)).toEqual([
      "name",
      "maxFileSizeBytes",
      "allowedMimeTypes",
      "backend",
    ]);
    expect(
      resolveFileStorageBucket({ ...form(), maxFileSizeBytes: 1.5 }, true).errors,
    ).toHaveLength(1);
  });
  it("requires S3 credentials on creation and preserves stored secrets on edit", () => {
    const s3Form = { ...form(), backend: "s3", endpoint: "invalid", region: "", accessKeyId: "" };
    expect(resolveFileStorageBucket(s3Form, true).errors.map((error) => error.name)).toEqual([
      "endpoint",
      "region",
      "accessKeyId",
      "secretAccessKey",
    ]);
    Object.assign(s3Form, {
      endpoint: "https://s3.example.com",
      region: " us-east-1 ",
      upstreamBucket: " remote-assets ",
      accessKeyId: " access ",
      secretAccessKey: "secret",
    });
    expect(resolveFileStorageBucket(s3Form, true)).toMatchObject({
      errors: [],
      input: {
        backendConfig: {
          endpoint: "https://s3.example.com",
          bucket: "remote-assets",
          region: "us-east-1",
          access_key_id: "access",
          secret_access_key: "secret",
        },
      },
    });
    s3Form.secretAccessKey = "";
    s3Form.endpoint = "";
    const editedFileStorageBucket = resolveFileStorageBucket(s3Form, false);
    expect(editedFileStorageBucket.errors).toEqual([]);
    expect(editedFileStorageBucket.input.backendConfig).not.toHaveProperty("secret_access_key");
    expect(editedFileStorageBucket.input.backendConfig).toHaveProperty("endpoint", "");
  });
});

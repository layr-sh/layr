import type { FormError } from "@nuxt/ui";
import type { FileStorageControlPlaneModels } from "layr-client";

export function fileStorageBytes(size: number) {
  const index = Math.min(4, Math.floor(Math.log2(Math.max(1, size)) / 10));
  return `${(size / 1024 ** index).toLocaleString(undefined, { maximumFractionDigits: 1 })} ${["B", "KiB", "MiB", "GiB", "TiB"][index]}`;
}
export function validFileStorageKey(key: string) {
  return (
    !!key &&
    !key.includes("\\") &&
    [...key].every((character) => character.charCodeAt(0) >= 32) &&
    key.split("/").every((part) => !!part && part !== "." && part !== "..")
  );
}
export function fileStorageMimeTypes(values: string[]) {
  return [...new Set(values.map((value) => value.trim().toLowerCase()).filter(Boolean))];
}
export function validFileStorageMimeTypes(values: string[]) {
  return values.every(
    (value) => value === "*/*" || /^[a-z0-9!#$&^_.+-]+\/(?:[a-z0-9!#$&^_.+-]+|\*)$/.test(value),
  );
}
export interface FileStorageBucketForm {
  name: string;
  backend: string;
  isPublic: boolean;
  maxFileSizeBytes: number;
  allowedMimeTypes: string[];
  endpoint: string;
  region: string;
  upstreamBucket: string;
  accessKeyId: string;
  secretAccessKey: string;
}
export function resolveFileStorageBucket(form: FileStorageBucketForm, creating: boolean) {
  const errors: FormError[] = [];
  if (!/^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/.test(form.name.trim()))
    errors.push({
      name: "name",
      message:
        "Use 3–63 lowercase letters, numbers, or hyphens, starting and ending with a letter or number.",
    });
  if (!Number.isSafeInteger(form.maxFileSizeBytes) || form.maxFileSizeBytes < 1)
    errors.push({ name: "maxFileSizeBytes", message: "Enter a positive whole number of bytes." });
  const allowedMimeTypes = fileStorageMimeTypes(form.allowedMimeTypes);
  if (!validFileStorageMimeTypes(allowedMimeTypes))
    errors.push({
      name: "allowedMimeTypes",
      message: "Use MIME types such as image/png, image/*, or */*.",
    });
  const input: FileStorageControlPlaneModels.CreateBucketInput = {
    name: form.name.trim(),
    backend: form.backend,
    isPublic: form.isPublic,
    maxFileSizeBytes: form.maxFileSizeBytes,
    allowedMimeTypes,
  };
  if (form.backend !== "database" && form.backend !== "s3")
    errors.push({ name: "backend", message: "Choose Database or S3." });
  if (form.backend === "s3") {
    if (form.endpoint && !/^https?:\/\/[^\s]+$/.test(form.endpoint.trim()))
      errors.push({
        name: "endpoint",
        message: "Enter an HTTP or HTTPS endpoint, or leave blank for AWS S3.",
      });
    if (!form.region.trim())
      errors.push({ name: "region", message: "Enter the upstream S3 region." });
    if (!form.accessKeyId.trim())
      errors.push({ name: "accessKeyId", message: "Enter the upstream access key ID." });
    if (creating && !form.secretAccessKey)
      errors.push({ name: "secretAccessKey", message: "Enter the upstream secret access key." });
    input.backendConfig = {
      endpoint: form.endpoint.trim(),
      region: form.region.trim(),
      bucket: form.upstreamBucket.trim(),
      access_key_id: form.accessKeyId.trim(),
      ...(form.secretAccessKey ? { secret_access_key: form.secretAccessKey } : {}),
    };
  }
  return { input, errors };
}

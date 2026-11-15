import type { LayrClient } from "../core/client";
import { useFileStorage, type FileStorageService } from "../services/file-storage";
import type { HttpFile } from "../../generated/file_storage/base/runtime";

export interface UploadOptions {
  contentType?: string;
}

export interface PresignedUrlOptions {
  operation?: "read" | "write";
  expiresInSeconds?: number;
}

export class BucketClient {
  private service: FileStorageService;
  readonly bucket: string;

  constructor(client: LayrClient, bucket: string) {
    this.service = useFileStorage(client);
    this.bucket = bucket;
  }

  async download(key: string): Promise<HttpFile> {
    const cleanKey = key.replace(/^\/+/, "");
    return await this.service.objects.download(this.bucket, cleanKey);
  }

  async head(key: string): Promise<Headers> {
    const cleanKey = key.replace(/^\/+/, "");
    return await this.service.objects.head(this.bucket, cleanKey);
  }

  async upload(
    key: string,
    file: Blob | ArrayBuffer | string,
    options: UploadOptions = {},
  ): Promise<void> {
    const cleanKey = key.replace(/^\/+/, "");
    await this.service.objects.upload_put(this.bucket, cleanKey, file, options);
  }

  async delete(key: string): Promise<void> {
    const cleanKey = key.replace(/^\/+/, "");
    await this.service.objects.delete(this.bucket, cleanKey);
  }

  async createPresignedURL(key: string, options: PresignedUrlOptions = {}): Promise<string> {
    const cleanKey = key.replace(/^\/+/, "");
    const response = await this.service.url.presign({
      bucket: this.bucket,
      key: cleanKey,
      operation: options.operation ?? "read",
      expiresInSeconds: options.expiresInSeconds ?? 3600,
    });
    return response.url;
  }
}

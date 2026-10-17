import * as crypto from 'node:crypto';
import type { LayrClient } from '../core/client';

export interface ResizeOptions {
  width?: number;
  height?: number;
  fit?: 'fit' | 'fill' | 'fill-down' | 'force' | 'auto';
  position?: string;
}

export function signImagePath(key: string | Buffer, salt: string | Buffer = '', path: string): string {
  const keyBuf = typeof key === 'string' ? Buffer.from(key) : key;
  const saltBuf = typeof salt === 'string' ? Buffer.from(salt) : salt;
  const pathBuf = Buffer.from(path);

  const hmac = crypto.createHmac('sha256', keyBuf);
  hmac.update(saltBuf);
  hmac.update(pathBuf);
  return hmac.digest('base64url');
}

export class ImageURLBuilder {
  private client: LayrClient;
  private source: string;
  private segments: string[] = [];
  private signature: string = '_';

  constructor(client: LayrClient, source: string) {
    this.client = client;
    this.source = source.replace(/^\/+/, '');
  }

  resize(options: ResizeOptions): this {
    const fit = options.fit || 'fit';
    const width = options.width ?? 0;
    const height = options.height ?? 0;
    this.segments.push(`rs:${fit}:${width}:${height}`);
    return this;
  }

  format(format: 'webp' | 'avif' | 'png' | 'jpeg' | string): this {
    this.segments.push(`f:${format}`);
    return this;
  }

  quality(quality: number): this {
    this.segments.push(`q:${quality}`);
    return this;
  }

  blur(sigma: number): this {
    this.segments.push(`bl:${sigma}`);
    return this;
  }

  rotate(degrees: number): this {
    this.segments.push(`rot:${degrees}`);
    return this;
  }

  flip(): this {
    this.segments.push('flip:1');
    return this;
  }

  flop(): this {
    this.segments.push('flop:1');
    return this;
  }

  sign(key: string, salt: string = ''): this {
    const relativePath = this.getRelativePath();
    this.signature = signImagePath(key, salt, relativePath);
    return this;
  }

  private getRelativePath(): string {
    const optionsPath = this.segments.length > 0 ? `${this.segments.join('/')}/` : '';
    return `/${optionsPath}plain/${this.source}`;
  }

  build(): string {
    const relativePath = this.getRelativePath();
    return `${this.client.endpoint}/v1/image/${this.signature}${relativePath}`;
  }

  toString(): string {
    return this.build();
  }
}

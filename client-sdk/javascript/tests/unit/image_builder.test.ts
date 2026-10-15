import { describe, it, expect } from 'vitest';
import { createClient } from '../../src/core/client';
import { ImageURLBuilder, signImagePath } from '../../src/extensions/image-url-builder';
import { useImage } from '../../src/services/image';

describe('ImageURLBuilder Unit Tests (IM-01 to IM-05)', () => {
  const publishableKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
  const client = createClient({
    endpoint: 'https://api.layr.sh',
    publishableKey,
  });

  it('IM-01: Geometry: Resize sets rs options in path', () => {
    const builder = new ImageURLBuilder(client, 'photo.jpg');
    builder.resize({ width: 800, height: 600, fit: 'fill' });
    expect(builder.build()).toBe('https://api.layr.sh/v1/image/_/rs:fill:800:600/plain/photo.jpg');

    const defaultBuilder = new ImageURLBuilder(client, '/sub/photo.jpg');
    defaultBuilder.resize({});
    expect(defaultBuilder.build()).toBe('https://api.layr.sh/v1/image/_/rs:fit:0:0/plain/sub/photo.jpg');
  });

  it('IM-02: Format & Compression sets format and quality', () => {
    const builder = new ImageURLBuilder(client, 'photo.jpg');
    builder.format('webp').quality(85);
    expect(builder.build()).toBe('https://api.layr.sh/v1/image/_/f:webp/q:85/plain/photo.jpg');
  });

  it('IM-03: Filters: Blur & Rotate sets blur and rotation', () => {
    const builder = new ImageURLBuilder(client, 'photo.jpg');
    builder.blur(2.5).rotate(90);
    expect(builder.build()).toBe('https://api.layr.sh/v1/image/_/bl:2.5/rot:90/plain/photo.jpg');
  });

  it('IM-04: Orientation: Flip & Flop appends flip:1 and flop:1', () => {
    const builder = new ImageURLBuilder(client, 'photo.jpg');
    builder.flip().flop();
    expect(builder.build()).toBe('https://api.layr.sh/v1/image/_/flip:1/flop:1/plain/photo.jpg');
    expect(builder.toString()).toBe('https://api.layr.sh/v1/image/_/flip:1/flop:1/plain/photo.jpg');
  });

  it('IM-05: Cryptographic HMAC Signing produces base64url signature', () => {
    const builder = new ImageURLBuilder(client, 'photo.jpg');
    builder.resize({ width: 300 }).sign('my-hmac-key', 'my-salt');
    const url = builder.build();

    expect(url).not.toContain('/_/');
    expect(url).toContain('rs:fit:300:0/plain/photo.jpg');

    const manualSig = signImagePath('my-hmac-key', 'my-salt', '/rs:fit:300:0/plain/photo.jpg');
    expect(url).toBe(`https://api.layr.sh/v1/image/${manualSig}/rs:fit:300:0/plain/photo.jpg`);

    const imageService = useImage(client);
    const serviceUrl = imageService.url('avatar.png').build();
    expect(serviceUrl).toBe('https://api.layr.sh/v1/image/_/plain/avatar.png');
  });

  it('signImagePath supports Buffer key and salt', () => {
    const keyBuf = Buffer.from('my-hmac-key');
    const saltBuf = Buffer.from('my-salt');
    const sig1 = signImagePath('my-hmac-key', 'my-salt', '/test');
    const sig2 = signImagePath(keyBuf, saltBuf, '/test');
    expect(sig1).toBe(sig2);
  });
});

import { describe, it, expect } from 'vitest';
import { useImage } from '../../../src/services/image';
import { createMockServiceContext } from './helpers';

describe('Image Service Unit Tests', () => {
  it('exercises image base endpoints', async () => {
    const { client, getLastUrl } = createMockServiceContext({
      width: 800,
      height: 600,
      format: 'jpeg',
    });
    const image = useImage(client);

    const builder = image.url('photo.jpg');
    expect(builder.build()).toContain('photo.jpg');

    await image.info.get({ signature: 'sig', path: 'photo.jpg' });
    expect(getLastUrl()).toContain('/v1/image/info/sig/photo.jpg');

    await image.info.post('binary-data');
    expect(getLastUrl()).toContain('/v1/image/info');

    await image.transform.get({ signature: 'sig', path: 'photo.jpg' });
    expect(getLastUrl()).toContain('/v1/image/sig/photo.jpg');
  });

  it('exercises image control plane (.cp) endpoints', async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } = createMockServiceContext();
    const image = elevate(useImage(client));

    // config
    await image.cp.config.get();
    expect(getLastUrl()).toContain('/v1/_/image/config');
    expect(getLastInit()?.headers).toMatchObject({ 'Service-Account-Key': serviceAccountKey });

    await image.cp.config.update({});
    expect(getLastUrl()).toContain('/v1/_/image/config');

    // presets
    await image.cp.presets.create({ name: 'thumbnail', options: {} });
    expect(getLastUrl()).toContain('/v1/_/image/presets');

    await image.cp.presets.get('preset-1');
    expect(getLastUrl()).toContain('/v1/_/image/presets/preset-1');

    await image.cp.presets.list();
    expect(getLastUrl()).toContain('/v1/_/image/presets');

    await image.cp.presets.update('preset-1', { name: 'thumb-updated' });
    expect(getLastUrl()).toContain('/v1/_/image/presets/preset-1');

    await image.cp.presets.delete('preset-1');
    expect(getLastUrl()).toContain('/v1/_/image/presets/preset-1');

    // stats
    await image.cp.stats.get();
    expect(getLastUrl()).toContain('/v1/_/image/stats');

    // url sign
    await image.cp.url.sign({ path: 'avatar.png' });
    expect(getLastUrl()).toContain('/v1/_/image/url/sign');
  });
});

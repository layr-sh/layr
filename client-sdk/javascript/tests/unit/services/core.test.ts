import { describe, it, expect } from 'vitest';
import { useCore } from '../../../src/services/core';
import { createMockServiceContext } from './helpers';

describe('Core Service Unit Tests', () => {
  it('exercises base manifest endpoint', async () => {
    const { client, getLastUrl } = createMockServiceContext({
      project: { name: 'test-project' },
      enabled_services: ['data', 'auth'],
      server: { base_url: 'https://api.layr.sh' },
      publishable_key: 'pk_123',
    });

    const core = useCore(client);
    const manifest = await core.manifest();
    expect(manifest.project.name).toBe('test-project');
    expect(getLastUrl()).toContain('/v1/manifest');
  });

  it('exercises core control plane (.cp) endpoints', async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } = createMockServiceContext();
    const core = elevate(useCore(client));

    // event_hooks
    await core.cp.event_hooks.create({ event: 'user.created', targetUrl: 'https://webhook.site' });
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks');
    expect(getLastInit()?.headers).toMatchObject({ 'Service-Account-Key': serviceAccountKey });

    await core.cp.event_hooks.get('hook-1');
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks/hook-1');

    await core.cp.event_hooks.list({ limit: 10 });
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks');

    await core.cp.event_hooks.update('hook-1', { targetUrl: 'https://new.site' });
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks/hook-1');

    await core.cp.event_hooks.delete('hook-1');
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks/hook-1');

    // event_hooks deliveries
    await core.cp.event_hooks.deliveries.list('hook-1');
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks/hook-1/deliveries');

    await core.cp.event_hooks.deliveries.retry('hook-1', 'del-1');
    expect(getLastUrl()).toContain('/v1/_/core/event-hooks/hook-1/deliveries/del-1/retry');

    // events
    await core.cp.events.get('event-1');
    expect(getLastUrl()).toContain('/v1/_/core/events/event-1');

    await core.cp.events.list({ limit: 5 });
    expect(getLastUrl()).toContain('/v1/_/core/events');

    // service_accounts
    await core.cp.service_accounts.create({ name: 'worker', scopes: ['data:read'] });
    expect(getLastUrl()).toContain('/v1/_/core/service-accounts');

    await core.cp.service_accounts.get('sa-1');
    expect(getLastUrl()).toContain('/v1/_/core/service-accounts/sa-1');

    await core.cp.service_accounts.list();
    expect(getLastUrl()).toContain('/v1/_/core/service-accounts');

    await core.cp.service_accounts.update('sa-1', { name: 'worker-updated' });
    expect(getLastUrl()).toContain('/v1/_/core/service-accounts/sa-1');

    await core.cp.service_accounts.delete('sa-1');
    expect(getLastUrl()).toContain('/v1/_/core/service-accounts/sa-1');
  });
});

import { describe, it, expect } from 'vitest';
import { useTasks } from '../../../src/services/tasks';
import { createMockServiceContext } from './helpers';

describe('Tasks Service Unit Tests', () => {
  it('exercises tasks control plane (.cp) endpoints', async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } = createMockServiceContext();
    const tasks = elevate(useTasks(client));

    // config
    await tasks.cp.config.get();
    expect(getLastUrl()).toContain('/v1/_/tasks/config');
    expect(getLastInit()?.headers).toMatchObject({ 'Service-Account-Key': serviceAccountKey });

    await tasks.cp.config.update({});
    expect(getLastUrl()).toContain('/v1/_/tasks/config');

    // dlq
    await tasks.cp.dlq.list();
    expect(getLastUrl()).toContain('/v1/_/tasks/dlq');

    await tasks.cp.dlq.list({ limit: 10 });
    expect(getLastUrl()).toContain('/v1/_/tasks/dlq');

    await tasks.cp.dlq.purge('exec-1');
    expect(getLastUrl()).toContain('/v1/_/tasks/dlq/exec-1');

    await tasks.cp.dlq.retry('exec-1');
    expect(getLastUrl()).toContain('/v1/_/tasks/dlq/exec-1/retry');

    // executions
    await tasks.cp.executions.create({ jobName: 'sync-data', payload: {} });
    expect(getLastUrl()).toContain('/v1/_/tasks/executions');

    await tasks.cp.executions.list();
    expect(getLastUrl()).toContain('/v1/_/tasks/executions');

    await tasks.cp.executions.list({ limit: 20 });
    expect(getLastUrl()).toContain('/v1/_/tasks/executions');

    // jobs
    await tasks.cp.jobs.create({
      name: 'cleanup-job',
      schedule: '0 0 * * *',
      targetUrl: 'https://app.local/jobs/cleanup',
    });
    expect(getLastUrl()).toContain('/v1/_/tasks/jobs');

    await tasks.cp.jobs.get('job-1');
    expect(getLastUrl()).toContain('/v1/_/tasks/jobs/job-1');

    await tasks.cp.jobs.list();
    expect(getLastUrl()).toContain('/v1/_/tasks/jobs');

    await tasks.cp.jobs.update('job-1', { schedule: '0 1 * * *' });
    expect(getLastUrl()).toContain('/v1/_/tasks/jobs/job-1');

    await tasks.cp.jobs.delete('job-1');
    expect(getLastUrl()).toContain('/v1/_/tasks/jobs/job-1');

    // stats
    await tasks.cp.stats.get();
    expect(getLastUrl()).toContain('/v1/_/tasks/stats');
  });
});

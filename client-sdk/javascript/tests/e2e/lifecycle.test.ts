import { describe, it, expect } from 'vitest';
import * as fs from 'node:fs';
import * as path from 'node:path';
import {
  createClient,
  useAuth,
  useConsole,
  useCore,
  useData,
  useFileStorage,
  useFunction,
  useImage,
  useTasks,
  withServiceAccount,
} from '../../src';

function getServiceAccountKey(): string | undefined {
  if (process.env.LAYR__SERVICE_ACCOUNT_KEY) {
    return process.env.LAYR__SERVICE_ACCOUNT_KEY;
  }
  const candidatePaths = [
    path.resolve(process.cwd(), '../../.layr/initial-root-account.txt'),
    path.resolve(process.cwd(), '.layr/initial-root-account.txt'),
    path.resolve(process.cwd(), '../.layr/initial-root-account.txt'),
  ];
  for (const filePath of candidatePaths) {
    if (fs.existsSync(filePath)) {
      const content = fs.readFileSync(filePath, 'utf-8');
      const match = content.match(/Service Account Key:\s*([a-f0-9]{32,64})/i);
      if (match && match[1]) {
        return match[1];
      }
    }
  }
  return undefined;
}

describe.skipIf(!process.env.LAYR__SERVER__BASE_URL)('Layr Production E2E SDK Lifecycle', () => {
  const endpoint = process.env.LAYR__SERVER__BASE_URL ?? 'http://127.0.0.1:5297';
  let publishableKey: string;
  let serviceAccountKey: string | undefined;
  let userSessionToken: string;

  it('1. probes cluster manifest via HTTP and useCore', async () => {
    const response = await fetch(`${endpoint}/v1/manifest`);
    expect(response.status).toBe(200);
    const body = (await response.json()) as { publishable_key?: string; project?: { name?: string } };
    expect(body.project).toBeDefined();
    expect(body.publishable_key).toBeDefined();
    publishableKey = body.publishable_key!;

    const client = createClient({ endpoint, publishableKey });
    const core = useCore(client);
    const manifest = await core.manifest();
    expect(manifest.publishableKey).toBe(publishableKey);
    expect(manifest.project.name).toBeDefined();

    serviceAccountKey = getServiceAccountKey();
  });

  it('2. executes DDL table creation and schema reflection via elevated Data control plane', async () => {
    if (!serviceAccountKey) {
      console.warn('Skipping elevated DDL test: serviceAccountKey not found in .layr/initial-root-account.txt');
      return;
    }
    const client = createClient({ endpoint, publishableKey });
    const dataElevated = withServiceAccount(useData(client), { serviceAccountKey });

    const tableName = `e2e_tasks_${Date.now()}`;
    const createResult = await dataElevated.cp.tables.create({
      schema: 'public',
      name: tableName,
      columns: [
        { name: 'id', type: 'uuid', isPrimaryKey: true, isNullable: false, isUnique: true },
        { name: 'title', type: 'text', isPrimaryKey: false, isNullable: false, isUnique: false },
        { name: 'status', type: 'text', isPrimaryKey: false, isNullable: false, isUnique: false },
      ],
    });
    expect(createResult.table).toBe(tableName);

    const tableSchema = await dataElevated.cp.tables.get('public', tableName);
    expect(tableSchema.name).toBe(tableName);

    // CRUD on the newly created table
    const data = useData(client);
    const inserted = await data.from(tableName).insert({
      title: 'Deploy to Production',
      status: 'pending',
    });
    expect(inserted).toBeDefined();
    const insertedId = (inserted as any).id;
    expect(insertedId).toBeDefined();

    // Query back
    const fetched = await data.from(tableName).select('*').eq('id', insertedId);
    expect(Array.isArray(fetched)).toBe(true);
    expect(fetched.length).toBe(1);
    expect((fetched[0] as any).title).toBe('Deploy to Production');

    // Update
    const updated = await data.from(tableName).update({ status: 'completed' }, insertedId);
    expect(updated).toBeDefined();

    // Delete
    await data.from(tableName).delete(insertedId);
    const afterDelete = await data.from(tableName).select('*').eq('id', insertedId);
    expect(afterDelete.length).toBe(0);

    // Drop table
    await dataElevated.cp.tables.delete('public', tableName);
  });

  it('3. executes authentication, session management, and user profile journey', async () => {
    const client = createClient({ endpoint, publishableKey });
    const auth = useAuth(client);
    const uniqueEmail = `e2e-user-${Date.now()}@example.com`;
    const password = 'StrongPassword123!';

    const signUpResult = await auth.sign_up({
      email: uniqueEmail,
      password,
      phone: '',
      properties: { role: 'tester' },
    });
    expect(signUpResult.accessToken).toBeDefined();
    expect(signUpResult.user.email).toBe(uniqueEmail);

    userSessionToken = signUpResult.accessToken;

    // Mutate session on client
    client.setSession(userSessionToken);

    // Retrieve authenticated user profile
    const profile = await auth.user.get();
    expect(profile.email).toBe(uniqueEmail);
  });

  it('4. executes ephemeral Key-Value cache operations', async () => {
    const client = createClient({ endpoint, publishableKey });
    const data = useData(client);
    const key = `e2e_setting_${Date.now()}`;

    await data.kv.set(key, { value: 'enabled', ttl: 120 });
    const retrieved = await data.kv.get(key);
    expect(retrieved.value).toBe('enabled');

    await data.kv.delete(key);
  });

  it('5. executes GraphQL introspection & query execution', async () => {
    const client = createClient({ endpoint, publishableKey });
    const data = useData(client);

    const gqlResult = await data.graphql({
      query: '{ __schema { types { name } } }',
    });
    expect(gqlResult.data).toBeDefined();
  });

  it('6. executes storage bucket lifecycle, upload, head, presign, download, and delete', async () => {
    if (!serviceAccountKey) {
      console.warn('Skipping storage bucket test: serviceAccountKey not found');
      return;
    }
    const client = createClient({ endpoint, publishableKey });
    if (userSessionToken) {
      client.setSession(userSessionToken);
    }
    const storageElevated = withServiceAccount(useFileStorage(client), { serviceAccountKey });

    const bucketName = `e2e-bucket-${Date.now()}`;
    const createdBucket = await storageElevated.cp.buckets.create({
      name: bucketName,
      backend: 'database',
      isPublic: true,
      maxFileSizeBytes: 5242880,
      allowedMimeTypes: ['text/plain'],
    });
    expect(createdBucket.name).toBe(bucketName);

    const bucketClient = useFileStorage(client).from(bucketName);
    const fileContent = 'Layr Enterprise Object Storage Payload';
    await bucketClient.upload('documents/readme.txt', new Blob([fileContent], { type: 'text/plain' }), {
      contentType: 'text/plain',
    });

    const headers = await bucketClient.head('documents/readme.txt');
    expect(headers).toBeDefined();

    const downloaded = await bucketClient.download('documents/readme.txt');
    const downloadedText = await downloaded.text();
    expect(downloadedText).toBe(fileContent);

    const presignedUrl = await bucketClient.createPresignedURL('documents/readme.txt', {
      operation: 'read',
      expiresInSeconds: 300,
    });
    expect(presignedUrl).toContain(bucketName);
    expect(presignedUrl).toContain('documents/readme.txt');

    await bucketClient.delete('documents/readme.txt');
    await storageElevated.cp.buckets.delete(bucketName);
  });

  it('7. executes Image transformation URL builder and HMAC signing', () => {
    const client = createClient({ endpoint, publishableKey });
    const image = useImage(client);

    const unsignedUrl = image
      .url('avatars/user.jpg')
      .resize({ width: 256, height: 256, fit: 'fill' })
      .format('webp')
      .quality(80)
      .blur(1.5)
      .build();

    expect(unsignedUrl).toContain('/v1/image/_/rs:fill:256:256/f:webp/q:80/bl:1.5/plain/avatars/user.jpg');

    const signedUrl = image
      .url('avatars/user.jpg')
      .resize({ width: 100, height: 100 })
      .sign('my-test-hmac-key-0123456789abcdef')
      .build();

    expect(signedUrl).not.toContain('/_/');
    expect(signedUrl).toContain('/v1/image/');
  });

  it('8. executes Tasks cron job scheduling, configuration, telemetry, and lifecycle', async () => {
    if (!serviceAccountKey) {
      console.warn('Skipping tasks test: serviceAccountKey not found');
      return;
    }
    const client = createClient({ endpoint, publishableKey });
    const tasksElevated = withServiceAccount(useTasks(client), { serviceAccountKey });

    // 1. Get runtime orchestrator configuration
    const config = await tasksElevated.cp.config.get();
    expect(config).toBeDefined();

    // 2. Register recurring cron job
    const jobName = `e2e_job_${Date.now()}`;
    const job = await tasksElevated.cp.jobs.create({
      name: jobName,
      cronExpression: '0 0 * * *',
      targetType: 'sql',
      targetPayload: { query: 'SELECT 1;' },
      isEnabled: true,
    });
    expect(job.name).toBe(jobName);
    expect(job.id).toBeDefined();

    // 3. Inspect job definition
    const fetchedJob = await tasksElevated.cp.jobs.get(job.id as string);
    expect(fetchedJob.id).toBe(job.id);
    expect(fetchedJob.name).toBe(jobName);

    // 4. List registered jobs
    const jobsList = await tasksElevated.cp.jobs.list();
    expect(jobsList.count).toBeGreaterThan(0);
    expect(jobsList.jobs?.some((j) => j.id === job.id)).toBe(true);

    // 5. Query execution telemetry and stats
    const stats = await tasksElevated.cp.stats.get();
    expect(stats).toBeDefined();

    // 6. Query executions and DLQ list
    const executions = await tasksElevated.cp.executions.list();
    expect(executions).toBeDefined();
    const dlq = await tasksElevated.cp.dlq.list();
    expect(dlq).toBeDefined();

    // 7. Cleanup job
    await tasksElevated.cp.jobs.delete(job.id as string);
  });

  it('9. executes Function serverless endpoint provisioning, deployment, and telemetry', async () => {
    if (!serviceAccountKey) {
      console.warn('Skipping function test: serviceAccountKey not found');
      return;
    }
    const client = createClient({ endpoint, publishableKey });
    const funcElevated = withServiceAccount(useFunction(client), { serviceAccountKey });

    // 1. Probe public proxy route for missing endpoint (returns 404)
    const proxyRes = await fetch(`${endpoint}/v1/function/e2e_missing_${Date.now()}`);
    expect(proxyRes.status).toBe(404);

    // 2. Get function runtime configuration
    const config = await funcElevated.cp.config.get();
    expect(config).toBeDefined();

    // 3. Create endpoint definition
    const endpointName = `e2e_func_${Date.now()}`;
    const createdEndpoint = await funcElevated.cp.endpoints.create({
      name: endpointName,
      description: 'E2E Test Function Endpoint',
      isPublic: true,
      runtime: 'workerd',
      entrypoint: 'index.js',
    });
    expect(createdEndpoint.name).toBe(endpointName);
    expect(createdEndpoint.id).toBeDefined();

    // 4. Inspect endpoint definition
    const fetchedEndpoint = await funcElevated.cp.endpoints.get(createdEndpoint.id as string);
    expect(fetchedEndpoint.id).toBe(createdEndpoint.id);
    expect(fetchedEndpoint.name).toBe(endpointName);

    // 5. List endpoints
    const endpointList = await funcElevated.cp.endpoints.list();
    expect(endpointList.count).toBeGreaterThan(0);
    expect(endpointList.endpoints?.some((ep) => ep.id === createdEndpoint.id)).toBe(true);

    // 6. Create deployment bundle
    const deployment = await funcElevated.cp.deployments.create(createdEndpoint.id as string, {
      bundleContent: 'export default { async fetch() { return new Response("OK"); } };',
    });
    expect(deployment.id).toBeDefined();
    expect(deployment.version).toBeGreaterThan(0);

    // 7. List deployments
    const deploymentsList = await funcElevated.cp.deployments.list(createdEndpoint.id as string);
    expect(deploymentsList.count).toBeGreaterThan(0);

    // 8. Query telemetry stats
    const stats = await funcElevated.cp.stats.get();
    expect(stats).toBeDefined();

    // 9. Cleanup endpoint
    await funcElevated.cp.endpoints.delete(createdEndpoint.id as string);
  });

  it('10. executes Console administrative configuration and operator identity management', async () => {
    if (!serviceAccountKey) {
      console.warn('Skipping console test: serviceAccountKey not found');
      return;
    }
    const client = createClient({ endpoint, publishableKey });
    const consoleElevated = withServiceAccount(useConsole(client), { serviceAccountKey });

    // 1. Get dynamic console runtime configuration
    const config = await consoleElevated.cp.config.get();
    expect(config).toBeDefined();

    // 2. Create operator user
    const operatorEmail = `e2e-operator-${Date.now()}@example.com`;
    const operatorPassword = 'OperatorPassword123!';
    const user = await consoleElevated.cp.users.create({
      email: operatorEmail,
      password: operatorPassword,
    });
    expect(user.id).toBeDefined();
    expect(user.email).toBe(operatorEmail);

    // 3. Inspect operator details
    const fetchedUser = await consoleElevated.cp.users.get(user.id);
    expect(fetchedUser.id).toBe(user.id);
    expect(fetchedUser.email).toBe(operatorEmail);

    // 4. List console users
    const usersList = await consoleElevated.cp.users.list();
    expect(usersList.some((u) => u.id === user.id)).toBe(true);

    // 5. Update operator account
    const updatedUser = await consoleElevated.cp.users.update(user.id, {
      isEnabled: true,
    });
    expect(updatedUser.id).toBe(user.id);

    // 6. List user sessions
    const sessions = await consoleElevated.cp.users.sessions.list(user.id);
    expect(Array.isArray(sessions)).toBe(true);

    // 7. Cleanup operator user
    await consoleElevated.cp.users.delete(user.id);
  });
});

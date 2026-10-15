import { describe, it, expect } from 'vitest';
import { useData } from '../../../src/services/data';
import { createMockServiceContext } from './helpers';

describe('Data Service Unit Tests', () => {
  it('exercises data base endpoints, kv, records, and rpc', async () => {
    let mockStatus = 200;
    const { client, getLastUrl, getLastInit } = createMockServiceContext((url, init) => {
      if (mockStatus === 204) {
        return new Response(null, { status: 204 });
      }
      if (url.includes('/graphql')) {
        return { data: { todos: [] } };
      }
      if (url.includes('/kv/')) {
        return { key: 'k1', value: 'v1' };
      }
      if (url.includes('/rpc/')) {
        return { result: 42 };
      }
      return [{ id: 'rec-1', name: 'Item 1' }];
    });

    const data = useData(client);

    // query builder & subscription
    const qb = data.from('todos');
    expect(qb).toBeDefined();

    const sub = data.subscribe('todos');
    expect(sub).toBeDefined();

    // graphql
    await data.graphql({ query: 'query { todos { id } }' });
    expect(getLastUrl()).toContain('/v1/graphql');

    // kv
    await data.kv.get('test-key');
    expect(getLastUrl()).toContain('/v1/data/kv/test-key');

    await data.kv.set('test-key', { value: 'val' });
    expect(getLastUrl()).toContain('/v1/data/kv/test-key');

    await data.kv.put('raw-key', 'raw-val', 60);
    expect(getLastUrl()).toContain('/v1/data/kv/raw-key');

    await data.kv.put('raw-obj', { a: 1 });
    expect(getLastUrl()).toContain('/v1/data/kv/raw-obj');

    await data.kv.increment({ key: 'counter', step: 1 });
    expect(getLastUrl()).toContain('/v1/data/kv/increment');

    await data.kv.mget({ keys: ['k1', 'k2'] });
    expect(getLastUrl()).toContain('/v1/data/kv/mget');

    await data.kv.mset({ entries: { k1: 'v1' } });
    expect(getLastUrl()).toContain('/v1/data/kv/mset');

    await data.kv.touch('k1', { ttl: 120 });
    expect(getLastUrl()).toContain('/v1/data/kv/k1');

    await data.kv.delete('k1');
    expect(getLastUrl()).toContain('/v1/data/kv/k1');

    // records
    await data.records.create('public', 'items', { name: 'Item 1' }, 'ignore');
    expect(getLastUrl()).toContain('/v1/data/public/items');
    expect(getLastUrl()).toContain('on_conflict=ignore');

    await data.records.create('public', 'items', [{ name: 'Item A' }]);
    expect(getLastUrl()).toContain('/v1/data/public/items');

    mockStatus = 204;
    const res204 = await data.records.create('public', 'items', { name: 'Item Void' });
    expect(res204).toBeUndefined();
    mockStatus = 200;

    await data.records.get('public', 'items', 'rec-1', 'id,name');
    expect(getLastUrl()).toContain('/v1/data/public/items/rec-1');
    expect(getLastUrl()).toContain('select=id%2Cname');

    await data.records.list('public', 'items');
    expect(getLastUrl()).toContain('/v1/data/public/items');

    await data.records.list('public', 'items', {
      limit: 10,
      offset: 5,
      select: 'id',
      order: 'id.desc',
      status: 'active',
    });
    expect(getLastUrl()).toContain('limit=10');
    expect(getLastUrl()).toContain('status=active');

    await data.records.put('public', 'items', 'rec-1', { name: 'Replaced' });
    expect(getLastUrl()).toContain('/v1/data/public/items/rec-1');
    expect(getLastInit()?.method).toBe('PUT');

    mockStatus = 204;
    const put204 = await data.records.put('public', 'items', 'rec-1', { name: 'Replaced' });
    expect(put204).toBeUndefined();
    mockStatus = 200;

    await data.records.update('public', 'items', 'rec-1', { name: 'Patched' });
    expect(getLastUrl()).toContain('/v1/data/public/items/rec-1');
    expect(getLastInit()?.method).toBe('PATCH');

    mockStatus = 204;
    const update204 = await data.records.update('public', 'items', 'rec-1', { name: 'Patched' });
    expect(update204).toBeUndefined();
    mockStatus = 200;

    await data.records.delete('public', 'items', 'rec-1');
    expect(getLastUrl()).toContain('/v1/data/public/items/rec-1');
    expect(getLastInit()?.method).toBe('DELETE');

    // rpc
    await data.rpc.query('public', 'get_stats', 'public.items');
    expect(getLastUrl()).toContain('/v1/data/public/rpc/get_stats');

    await data.rpc.query('public', 'get_stats');
    expect(getLastUrl()).toContain('/v1/data/public/rpc/get_stats');

    await data.rpc.mutation('public', 'process_order', { order_id: 123 }, 'public.orders');
    expect(getLastUrl()).toContain('/v1/data/public/rpc/process_order');

    await data.rpc.mutation('public', 'process_order', { order_id: 123 });
    expect(getLastUrl()).toContain('/v1/data/public/rpc/process_order');
  });

  it('exercises data control plane (.cp) endpoints', async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } = createMockServiceContext();
    const data = elevate(useData(client));

    // cache
    await data.cp.cache.flush();
    expect(getLastUrl()).toContain('/v1/_/data/cache/flush');
    expect(getLastInit()?.headers).toMatchObject({ 'Service-Account-Key': serviceAccountKey });

    await data.cp.cache.invalidate({ tables: ['public.items'] });
    expect(getLastUrl()).toContain('/v1/_/data/cache/invalidate');

    // config
    await data.cp.config.get();
    expect(getLastUrl()).toContain('/v1/_/data/config');

    await data.cp.config.update({});
    expect(getLastUrl()).toContain('/v1/_/data/config');

    // tables
    await data.cp.tables.create({ name: 'users' });
    expect(getLastUrl()).toContain('/v1/_/data/tables');

    await data.cp.tables.get('public', 'users');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users');

    await data.cp.tables.list();
    expect(getLastUrl()).toContain('/v1/_/data/tables');

    await data.cp.tables.delete('public', 'users');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users');

    // columns
    await data.cp.tables.columns.create('public', 'users', { name: 'email', type: 'text' });
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/columns');

    await data.cp.tables.columns.update('public', 'users', 'email', { name: 'contact_email' });
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/columns/email');

    await data.cp.tables.columns.delete('public', 'users', 'contact_email');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/columns/contact_email');

    // indexes
    await data.cp.tables.indexes.create('public', 'users', { name: 'idx_users_email', columns: ['email'] });
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/indexes');

    await data.cp.tables.indexes.list('public', 'users');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/indexes');

    await data.cp.tables.indexes.delete('public', 'users', 'idx_users_email');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/indexes/idx_users_email');

    // policies
    await data.cp.tables.policies.create('public', 'users', { name: 'user_isolation', command: 'ALL' });
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/policies');

    await data.cp.tables.policies.list('public', 'users');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/policies');

    await data.cp.tables.policies.delete('public', 'users', 'user_isolation');
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/policies/user_isolation');

    // rls
    await data.cp.tables.rls.update('public', 'users', { enabled: true }, 'enable', true);
    expect(getLastUrl()).toContain('/v1/_/data/tables/public/users/rls');
    expect(getLastUrl()).toContain('action=enable');
    expect(getLastUrl()).toContain('force=true');
  });

  it('normalizes wrapped and unwrapped records responses in list, put, and update', async () => {
    let mockResponse: any = { data: [{ id: 'item-1' }] };
    const { client } = createMockServiceContext(() => mockResponse);
    const data = useData(client);

    // 1. list with { data: [...] }
    const listWrapped = await data.records.list('public', 'items');
    expect(listWrapped).toEqual([{ id: 'item-1' }]);

    // 2. list with raw object (not array, no data)
    mockResponse = { fallback: true };
    const listFallback = await data.records.list('public', 'items');
    expect(listFallback).toEqual([{ fallback: true }]);

    // 3. put with { data: { ... } }
    mockResponse = { data: { id: 'rec-put', name: 'PutData' } };
    const putWrapped = await data.records.put('public', 'items', 'rec-put', {});
    expect(putWrapped).toEqual({ id: 'rec-put', name: 'PutData' });

    // 4. put with raw object (no data field)
    mockResponse = { id: 'rec-put-raw', name: 'PutRaw' };
    const putRaw = await data.records.put('public', 'items', 'rec-put-raw', {});
    expect(putRaw).toEqual({ id: 'rec-put-raw', name: 'PutRaw' });

    // 5. update with { data: { ... } }
    mockResponse = { data: { id: 'rec-patch', name: 'PatchData' } };
    const updateWrapped = await data.records.update('public', 'items', 'rec-patch', {});
    expect(updateWrapped).toEqual({ id: 'rec-patch', name: 'PatchData' });

    // 6. update with raw object (no data field)
    mockResponse = { id: 'rec-patch-raw', name: 'PatchRaw' };
    const updateRaw = await data.records.update('public', 'items', 'rec-patch-raw', {});
    expect(updateRaw).toEqual({ id: 'rec-patch-raw', name: 'PatchRaw' });

    // 7. create with wrapped data object
    mockResponse = { data: { id: 'rec-create', name: 'Created' } };
    const createWrapped = await data.records.create('public', 'items', { name: 'Created' });
    expect(createWrapped).toEqual({ id: 'rec-create', name: 'Created' });

    // 8. get with wrapped data object
    mockResponse = { data: { id: 'rec-get', name: 'Got' } };
    const getWrapped = await data.records.get('public', 'items', 'rec-get');
    expect(getWrapped).toEqual({ id: 'rec-get', name: 'Got' });

    // 9. data.subscribe reusing existing singleton
    const sub1 = data.subscribe('todos');
    const sub2 = data.subscribe('todos');
    expect(sub1).toBeDefined();
    expect(sub2).toBeDefined();
  });
});

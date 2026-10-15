import { describe, it, expect, vi } from 'vitest';
import { createClient } from '../../src/core/client';
import { BucketClient } from '../../src/extensions/bucket-client';
import { useFileStorage } from '../../src/services/file-storage';
import { LayrAPIError } from '../../src/core/errors';

describe('BucketClient Unit Tests (BK-01 to BK-05)', () => {
  const publishableKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
  const fetchHandler = vi.fn();
  const client = createClient({
    endpoint: 'https://api.layr.sh',
    publishableKey,
    fetch: (input, requestInit) => fetchHandler(input, requestInit),
  });
  const bucket = new BucketClient(client, 'docs');

  it('BK-01: File Upload with explicit content type and Blob inference', async () => {
    let capturedUrl = '';
    let capturedOptions: RequestInit | undefined;

    fetchHandler.mockImplementation(async (url: string, requestOptions: RequestInit) => {
      capturedUrl = url;
      capturedOptions = requestOptions;
      return new Response('{}', { status: 200 });
    });

    await bucket.upload('/contract.pdf', 'test data', { contentType: 'application/pdf' });
    expect(capturedUrl).toBe('https://api.layr.sh/v1/file-storage/objects/docs/contract.pdf');
    expect(capturedOptions?.method).toBe('PUT');
    expect((capturedOptions?.headers as Record<string, string>)['Content-Type']).toBe('application/pdf');

    const blob = new Blob(['blob data'], { type: 'image/png' });
    await bucket.upload('image.png', blob);
    expect((capturedOptions?.headers as Record<string, string>)['Content-Type']).toBe('image/png');

    fetchHandler.mockResolvedValue(
      new Response(JSON.stringify({ detail: 'Upload forbidden' }), { status: 403, statusText: 'Forbidden' })
    );
    await expect(bucket.upload('file.txt', 'fail')).rejects.toThrow(LayrAPIError);
  });

  it('BK-02: File Download returns Blob and throws LayrAPIError on failure', async () => {
    fetchHandler.mockResolvedValue(
      new Response('binary data content', { status: 200, headers: { 'Content-Type': 'application/octet-stream' } })
    );

    const blob = await bucket.download('contract.pdf');
    expect(blob).toBeInstanceOf(Blob);
    expect(await blob.text()).toBe('binary data content');

    fetchHandler.mockResolvedValue(
      new Response(JSON.stringify({ detail: 'Not found' }), { status: 404, statusText: 'Not Found' })
    );
    await expect(bucket.download('missing.pdf')).rejects.toThrow(LayrAPIError);
  });

  it('BK-03: File Metadata (HEAD) returns Headers and handles errors', async () => {
    fetchHandler.mockResolvedValue(
      new Response(null, { status: 200, headers: { 'Content-Length': '1024', 'Content-Type': 'application/pdf' } })
    );

    const headers = await bucket.head('contract.pdf');
    expect(headers.get('Content-Length')).toBe('1024');

    fetchHandler.mockResolvedValue(
      new Response(null, { status: 404, statusText: 'Not Found' })
    );
    await expect(bucket.head('missing.pdf')).rejects.toThrow(LayrAPIError);
  });

  it('BK-04: File Deletion sends DELETE request and handles errors', async () => {
    let capturedMethod = '';
    fetchHandler.mockImplementation(async (_url: string, requestOptions: RequestInit) => {
      capturedMethod = requestOptions.method ?? '';
      return new Response(null, { status: 204 });
    });

    await bucket.delete('contract.pdf');
    expect(capturedMethod).toBe('DELETE');

    fetchHandler.mockResolvedValue(
      new Response(JSON.stringify({ detail: 'Delete failed' }), { status: 500 })
    );
    await expect(bucket.delete('error.pdf')).rejects.toThrow(LayrAPIError);
  });

  it('BK-05: Presigned URL Generation supports expiresInSeconds parameter in seconds', async () => {
    let capturedBody = '';
    fetchHandler.mockImplementation(async (_url: string, requestOptions: RequestInit) => {
      capturedBody = requestOptions.body as string;
      return new Response(JSON.stringify({ url: 'https://storage.layr.sh/signed/123' }), { status: 200 });
    });

    const url1 = await bucket.createPresignedURL('contract.pdf', { operation: 'write', expiresInSeconds: 7200 });
    expect(url1).toBe('https://storage.layr.sh/signed/123');
    expect(JSON.parse(capturedBody)).toEqual({
      bucket: 'docs',
      key: 'contract.pdf',
      operation: 'write',
      expires_in_seconds: 7200,
    });

    const url2 = await bucket.createPresignedURL('contract.pdf');
    expect(url2).toBe('https://storage.layr.sh/signed/123');
    expect(JSON.parse(capturedBody)).toEqual({
      bucket: 'docs',
      key: 'contract.pdf',
      operation: 'read',
      expires_in_seconds: 3600,
    });

    const storage = useFileStorage(client);
    expect(storage.from('docs')).toBeInstanceOf(BucketClient);

    fetchHandler.mockResolvedValue(
      new Response(JSON.stringify({ detail: 'Presign failed' }), { status: 400 })
    );
    await expect(bucket.createPresignedURL('bad.pdf')).rejects.toThrow(LayrAPIError);
  });
});

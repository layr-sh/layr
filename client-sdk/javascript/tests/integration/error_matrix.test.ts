import { describe, it, expect } from 'vitest';
import { LayrAPIError } from '../../src/core/errors';

describe('Error Matrix Integration Tests (ERR-01 to ERR-09)', () => {
  it('ERR-01: 400 Bad Request (RFC 9457 Problem Details)', async () => {
    const response = new Response(
      JSON.stringify({
        title: 'Bad Request',
        status: 400,
        detail: 'Invalid UUID',
        instance: '/v1/data/public/users/123',
      }),
      { status: 400, statusText: 'Bad Request', headers: { 'Content-Type': 'application/problem+json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(400);
    expect(err.title).toBe('Bad Request');
    expect(err.detail).toBe('Invalid UUID');
    expect(err.message).toBe('Invalid UUID');
    expect(err.instance).toBe('/v1/data/public/users/123');
  });

  it('ERR-02: 401 Unauthorized (RFC 6749 OAuth Error)', async () => {
    const response = new Response(
      JSON.stringify({
        error: 'invalid_grant',
        error_description: 'Token expired',
      }),
      { status: 401, statusText: 'Unauthorized', headers: { 'Content-Type': 'application/json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(401);
    expect(err.error).toBe('invalid_grant');
    expect(err.error_description).toBe('Token expired');
    expect(err.detail).toBe('Token expired');
    expect(err.message).toBe('Token expired');
  });

  it('ERR-03: 403 Forbidden (RFC 9457)', async () => {
    const response = new Response(
      JSON.stringify({
        title: 'Forbidden',
        status: 403,
        detail: 'Insufficient permissions',
      }),
      { status: 403, statusText: 'Forbidden', headers: { 'Content-Type': 'application/problem+json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(403);
    expect(err.title).toBe('Forbidden');
    expect(err.detail).toBe('Insufficient permissions');
  });

  it('ERR-04: 404 Not Found (RFC 9457)', async () => {
    const response = new Response(
      JSON.stringify({
        title: 'Not Found',
        status: 404,
        detail: 'Resource not found',
      }),
      { status: 404, statusText: 'Not Found', headers: { 'Content-Type': 'application/problem+json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(404);
    expect(err.title).toBe('Not Found');
    expect(err.detail).toBe('Resource not found');
  });

  it('ERR-05: 409 Conflict (RFC 9457)', async () => {
    const response = new Response(
      JSON.stringify({
        title: 'Conflict',
        status: 409,
        detail: 'Unique constraint violation',
      }),
      { status: 409, statusText: 'Conflict', headers: { 'Content-Type': 'application/problem+json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(409);
    expect(err.title).toBe('Conflict');
    expect(err.detail).toBe('Unique constraint violation');
  });

  it('ERR-06: 422 Unprocessable Entity (RFC 9457)', async () => {
    const response = new Response(
      JSON.stringify({
        title: 'Unprocessable Entity',
        status: 422,
        detail: 'Validation failed',
      }),
      { status: 422, statusText: 'Unprocessable Entity', headers: { 'Content-Type': 'application/problem+json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(422);
    expect(err.title).toBe('Unprocessable Entity');
    expect(err.detail).toBe('Validation failed');
  });

  it('ERR-07: 500 Internal Server Error (RFC 9457)', async () => {
    const response = new Response(
      JSON.stringify({
        title: 'Internal Error',
        status: 500,
        detail: 'Database error',
      }),
      { status: 500, statusText: 'Internal Server Error', headers: { 'Content-Type': 'application/problem+json' } }
    );

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(500);
    expect(err.title).toBe('Internal Error');
    expect(err.detail).toBe('Database error');
  });

  it('ERR-08: 502 Bad Gateway (HTML body fallback)', async () => {
    const response = new Response('<html><body>502 Bad Gateway</body></html>', {
      status: 502,
      statusText: 'Bad Gateway',
      headers: { 'Content-Type': 'text/html' },
    });

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(502);
    expect(err.detail).toBe('<html><body>502 Bad Gateway</body></html>');
    expect(err.message).toBe('<html><body>502 Bad Gateway</body></html>');
  });

  it('ERR-09: 500 Server Error (0 bytes empty body fallback)', async () => {
    const response = new Response('', {
      status: 500,
      statusText: 'Internal Server Error',
      headers: { 'Content-Type': 'text/plain' },
    });

    const err = await LayrAPIError.fromResponse(response);
    expect(err.status).toBe(500);
    expect(err.detail).toBe('Internal Server Error');
    expect(err.message).toBe('Internal Server Error');
  });
});

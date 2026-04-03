import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchLobbySessions, createSession } from './api';

function makeResponse(ok: boolean, body: unknown): Response {
  const text = typeof body === 'string' ? body : JSON.stringify(body);
  return {
    ok,
    status: ok ? 200 : 400,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(text),
  } as unknown as Response;
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

describe('fetchLobbySessions', () => {
  it('returns parsed JSON on a 200 response', async () => {
    const sessions = [{ id: 's1', goal: 500, hostUsername: 'alice', writerCount: 1 }];
    vi.mocked(fetch).mockResolvedValue(makeResponse(true, sessions));
    await expect(fetchLobbySessions()).resolves.toEqual(sessions);
  });

  it('throws an error with the response body text on a non-OK response', async () => {
    vi.mocked(fetch).mockResolvedValue(makeResponse(false, 'session not found'));
    await expect(fetchLobbySessions()).rejects.toThrow('session not found');
  });

  it('propagates network errors', async () => {
    vi.mocked(fetch).mockRejectedValue(new Error('network failure'));
    await expect(fetchLobbySessions()).rejects.toThrow('network failure');
  });
});

describe('createSession', () => {
  it('sends a POST with correct headers and JSON body', async () => {
    const response = { id: 's1', participantId: 'p1' };
    vi.mocked(fetch).mockResolvedValue(makeResponse(true, response));
    await createSession({ goal: 500, username: 'alice', public: true });
    expect(fetch).toHaveBeenCalledWith('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ goal: 500, username: 'alice', public: true }),
    });
  });

  it('returns parsed JSON on a 200 response', async () => {
    const response = { id: 's1', participantId: 'p1' };
    vi.mocked(fetch).mockResolvedValue(makeResponse(true, response));
    await expect(createSession({ goal: 500, username: 'alice' })).resolves.toEqual(response);
  });

  it('throws an error with the response body text on a non-OK response', async () => {
    vi.mocked(fetch).mockResolvedValue(makeResponse(false, 'username taken'));
    await expect(createSession({ goal: 500, username: 'alice' })).rejects.toThrow('username taken');
  });
});

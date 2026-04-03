import type { CreateSessionRequest, CreateSessionResponse, LobbySession } from './types';

export async function fetchLobbySessions(): Promise<LobbySession[]> {
  const res = await fetch('/api/sessions');
  if (!res.ok) {
    const text = await res.text().catch(() => 'Unknown error');
    throw new Error(text.trim() || `Server error ${res.status}`);
  }
  return res.json();
}

export async function createSession(req: CreateSessionRequest): Promise<CreateSessionResponse> {
  const res = await fetch('/api/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });

  if (!res.ok) {
    const text = await res.text().catch(() => 'Unknown error');
    throw new Error(text.trim() || `Server error ${res.status}`);
  }

  return res.json();
}

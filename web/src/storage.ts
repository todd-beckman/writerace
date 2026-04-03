import type { SessionRecord } from './types';

const STORAGE_KEY = 'writerace:sessions';

export function generateSessionId(): string {
  return crypto.randomUUID().replace(/-/g, '').slice(0, 8);
}

export function listSessions(): SessionRecord[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]');
  } catch {
    return [];
  }
}

export function loadSession(id: string): SessionRecord | null {
  return listSessions().find((s) => s.id === id) ?? null;
}

export function saveSession(session: SessionRecord): void {
  const sessions = listSessions();
  const idx = sessions.findIndex((s) => s.id === session.id);
  if (idx >= 0) {
    sessions[idx] = session;
  } else {
    sessions.push(session);
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions));
}

export function deleteSession(id: string): void {
  const sessions = listSessions().filter((s) => s.id !== id);
  localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions));
}

export function clearAllSessions(): void {
  localStorage.removeItem(STORAGE_KEY);
}

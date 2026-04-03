const KEY = 'writerace:participants';
const USERNAME_KEY = 'writerace:last-username';

export interface StoredParticipant {
  participantId: string;
  username: string;
}

function load(): Record<string, StoredParticipant> {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? '{}');
  } catch {
    return {};
  }
}

export function getStoredParticipant(sessionId: string): StoredParticipant | null {
  return load()[sessionId] ?? null;
}

export function setStoredParticipant(sessionId: string, data: StoredParticipant): void {
  const all = load();
  all[sessionId] = data;
  localStorage.setItem(KEY, JSON.stringify(all));
  localStorage.setItem(USERNAME_KEY, data.username);
}

export function getLastUsername(): string {
  return localStorage.getItem(USERNAME_KEY) ?? '';
}

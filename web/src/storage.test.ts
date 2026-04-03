import { beforeEach, describe, expect, it, vi } from 'vitest';
import { listSessions, loadSession, saveSession, deleteSession, clearAllSessions } from './storage';
import type { SessionRecord } from './types';

function makeRecord(overrides: Partial<SessionRecord> = {}): SessionRecord {
  return {
    id: 'abc123',
    username: 'alice',
    goal: 500,
    writing: '',
    createdAt: 1000,
    ...overrides,
  };
}

function makeLocalStorage(): Storage {
  let store: Record<string, string> = {};
  return {
    getItem: (key) => store[key] ?? null,
    setItem: (key, value) => { store[key] = value; },
    removeItem: (key) => { delete store[key]; },
    clear: () => { store = {}; },
    get length() { return Object.keys(store).length; },
    key: (index) => Object.keys(store)[index] ?? null,
  };
}

beforeEach(() => {
  vi.stubGlobal('localStorage', makeLocalStorage());
});

describe('listSessions', () => {
  it('returns empty array when localStorage is empty', () => {
    expect(listSessions()).toEqual([]);
  });

  it('returns empty array when localStorage contains invalid JSON', () => {
    localStorage.setItem('writerace:sessions', 'not json');
    expect(listSessions()).toEqual([]);
  });

  it('returns the parsed array when valid data exists', () => {
    const records = [makeRecord()];
    localStorage.setItem('writerace:sessions', JSON.stringify(records));
    expect(listSessions()).toEqual(records);
  });
});

describe('loadSession', () => {
  it('returns null when no sessions exist', () => {
    expect(loadSession('abc123')).toBeNull();
  });

  it("returns null when the ID doesn't match", () => {
    saveSession(makeRecord({ id: 'abc123' }));
    expect(loadSession('zzz999')).toBeNull();
  });

  it('returns the matching session record', () => {
    const record = makeRecord({ id: 'abc123' });
    saveSession(record);
    expect(loadSession('abc123')).toEqual(record);
  });
});

describe('saveSession', () => {
  it('adds a new session when none exist', () => {
    saveSession(makeRecord({ id: 'abc123' }));
    expect(listSessions()).toHaveLength(1);
  });

  it('adds a new session to an existing list', () => {
    saveSession(makeRecord({ id: 'first' }));
    saveSession(makeRecord({ id: 'second' }));
    expect(listSessions()).toHaveLength(2);
  });

  it('updates an existing session by ID', () => {
    saveSession(makeRecord({ id: 'abc123', writing: 'original' }));
    saveSession(makeRecord({ id: 'abc123', writing: 'updated' }));
    const sessions = listSessions();
    expect(sessions).toHaveLength(1);
    expect(sessions[0].writing).toBe('updated');
  });
});

describe('deleteSession', () => {
  it('removes the session with the given ID', () => {
    saveSession(makeRecord({ id: 'abc123' }));
    deleteSession('abc123');
    expect(listSessions()).toHaveLength(0);
  });

  it("does nothing if the ID doesn't exist", () => {
    saveSession(makeRecord({ id: 'abc123' }));
    deleteSession('zzz999');
    expect(listSessions()).toHaveLength(1);
  });

  it('leaves other sessions intact', () => {
    saveSession(makeRecord({ id: 'keep' }));
    saveSession(makeRecord({ id: 'remove' }));
    deleteSession('remove');
    const sessions = listSessions();
    expect(sessions).toHaveLength(1);
    expect(sessions[0].id).toBe('keep');
  });
});

describe('clearAllSessions', () => {
  it('removes the storage key entirely', () => {
    saveSession(makeRecord());
    clearAllSessions();
    expect(localStorage.getItem('writerace:sessions')).toBeNull();
  });

  it('listSessions returns empty array afterward', () => {
    saveSession(makeRecord());
    clearAllSessions();
    expect(listSessions()).toEqual([]);
  });
});

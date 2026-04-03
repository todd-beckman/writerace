import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getStoredParticipant, setStoredParticipant, getLastUsername } from './participant-store';
import type { StoredParticipant } from './participant-store';

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

describe('getStoredParticipant', () => {
  it('returns null when no data exists', () => {
    expect(getStoredParticipant('session1')).toBeNull();
  });

  it('returns null for an unknown session ID', () => {
    setStoredParticipant('session1', { participantId: 'p1', username: 'alice' });
    expect(getStoredParticipant('unknown')).toBeNull();
  });

  it('returns the stored participant for a known session ID', () => {
    const data: StoredParticipant = { participantId: 'p1', username: 'alice' };
    setStoredParticipant('session1', data);
    expect(getStoredParticipant('session1')).toEqual(data);
  });
});

describe('setStoredParticipant', () => {
  it('stores a participant retrievable by session ID', () => {
    setStoredParticipant('session1', { participantId: 'p1', username: 'alice' });
    expect(getStoredParticipant('session1')).toEqual({ participantId: 'p1', username: 'alice' });
  });

  it('overwrites a previous entry for the same session ID', () => {
    setStoredParticipant('session1', { participantId: 'p1', username: 'alice' });
    setStoredParticipant('session1', { participantId: 'p2', username: 'bob' });
    expect(getStoredParticipant('session1')).toEqual({ participantId: 'p2', username: 'bob' });
  });

  it('also saves the username as the last username', () => {
    setStoredParticipant('session1', { participantId: 'p1', username: 'alice' });
    expect(getLastUsername()).toBe('alice');
  });
});

describe('getLastUsername', () => {
  it('returns empty string when no username has been saved', () => {
    expect(getLastUsername()).toBe('');
  });

  it('returns the most recently saved username', () => {
    setStoredParticipant('session1', { participantId: 'p1', username: 'alice' });
    setStoredParticipant('session2', { participantId: 'p2', username: 'bob' });
    expect(getLastUsername()).toBe('bob');
  });
});

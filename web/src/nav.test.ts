import { beforeEach, describe, expect, it, vi } from 'vitest';
import { setNavigationGuard, getNavigationGuard, clearNavigationGuard, navigate } from './nav';

beforeEach(() => {
  clearNavigationGuard();
  vi.stubGlobal('window', { location: { hash: '' } });
});

describe('setNavigationGuard / getNavigationGuard / clearNavigationGuard', () => {
  it('getNavigationGuard returns null initially', () => {
    expect(getNavigationGuard()).toBeNull();
  });

  it('getNavigationGuard returns the function after setNavigationGuard', () => {
    const guard = () => true;
    setNavigationGuard(guard);
    expect(getNavigationGuard()).toBe(guard);
  });

  it('getNavigationGuard returns null after clearNavigationGuard', () => {
    setNavigationGuard(() => true);
    clearNavigationGuard();
    expect(getNavigationGuard()).toBeNull();
  });
});

describe('navigate', () => {
  it('sets window.location.hash when no guard is set', () => {
    navigate('lobby');
    expect(window.location.hash).toBe('lobby');
  });

  it('sets the hash and clears the guard when the guard returns true', () => {
    setNavigationGuard(() => true);
    navigate('lobby');
    expect(window.location.hash).toBe('lobby');
    expect(getNavigationGuard()).toBeNull();
  });

  it('does not change the hash when the guard returns false', () => {
    window.location.hash = 'original';
    setNavigationGuard(() => false);
    navigate('lobby');
    expect(window.location.hash).toBe('original');
  });
});

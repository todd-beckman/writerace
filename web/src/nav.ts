type Guard = () => boolean;

let _guard: Guard | null = null;

export function setNavigationGuard(fn: Guard | null): void {
  _guard = fn;
}

export function getNavigationGuard(): Guard | null {
  return _guard;
}

export function clearNavigationGuard(): void {
  _guard = null;
}

export function navigate(hash: string): void {
  if (_guard) {
    if (!_guard()) return;
    _guard = null;
  }
  window.location.hash = hash;
}

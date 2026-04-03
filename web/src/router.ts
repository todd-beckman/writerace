import { getNavigationGuard, clearNavigationGuard } from './nav';
import * as lobbyPage from './pages/lobby';
import * as createPage from './pages/create';
import * as sessionPage from './pages/session';
import * as recoverPage from './pages/recover';
import * as privacyPage from './pages/privacy';

type Teardown = () => void;

let _currentTeardown: Teardown | null = null;
let _prevHash = window.location.hash;
let _skipNext = false;

function parseHash(): { path: string; params: URLSearchParams } {
  const full = window.location.hash.slice(1); // remove leading '#'
  const [path = '', query = ''] = full.split('?');
  return { path: path || 'lobby', params: new URLSearchParams(query) };
}

function render(): void {
  const container = document.getElementById('page-content')!;

  if (_currentTeardown) {
    _currentTeardown();
    _currentTeardown = null;
  }

  const { path, params } = parseHash();

  if (path === 'lobby' || path === '') {
    lobbyPage.mount(container);
    _currentTeardown = lobbyPage.teardown;
  } else if (path === 'create') {
    createPage.mount(container, params);
    _currentTeardown = createPage.teardown;
  } else if (path === 'recover') {
    recoverPage.mount(container);
    _currentTeardown = recoverPage.teardown;
  } else if (path === 'privacy') {
    privacyPage.mount(container);
    _currentTeardown = privacyPage.teardown;
  } else {
    const sessionMatch = path.match(/^session\/(.+)$/);
    if (sessionMatch) {
      sessionPage.mount(container, sessionMatch[1]);
      _currentTeardown = sessionPage.teardown;
    } else {
      // Unknown route — fall back to lobby.
      lobbyPage.mount(container);
      _currentTeardown = lobbyPage.teardown;
    }
  }
}

export function initRouter(): void {
  window.addEventListener('hashchange', () => {
    if (_skipNext) {
      _skipNext = false;
      return;
    }

    const guard = getNavigationGuard();
    if (guard) {
      if (!guard()) {
        // Restore previous hash without triggering another render.
        _skipNext = true;
        window.location.hash = _prevHash;
        return;
      }
      clearNavigationGuard();
    }

    _prevHash = window.location.hash;
    render();
  });

  _prevHash = window.location.hash;
  render();
}

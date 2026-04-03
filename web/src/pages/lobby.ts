import { navigate } from '../nav';
import { fetchLobbySessions } from '../api';
import type { LobbySession } from '../types';

const POLL_INTERVAL = 4000;

let _pollTimer: ReturnType<typeof setInterval> | null = null;
let _container: HTMLElement | null = null;

export function mount(container: HTMLElement): void {
  _container = container;

  container.innerHTML = `
    <h1>Lobby</h1>
    <p class="subtitle" style="margin-bottom:16px">
      <button class="btn-primary btn-sm" id="lobby-create-btn">Create a Session</button>
    </p>
    <h2 style="margin-bottom:16px">Public Sessions</h2>
    <div id="lobby-list"></div>
  `;

  container.querySelector('#lobby-create-btn')!.addEventListener('click', () => navigate('create'));

  // Event delegation for Join buttons rendered inside #lobby-list.
  container.querySelector('#lobby-list')!.addEventListener('click', (e) => {
    const btn = (e.target as Element).closest<HTMLButtonElement>('[data-join-id]');
    if (btn) navigate(`session/${btn.dataset.joinId}`);
  });

  fetchAndRender();
  _pollTimer = setInterval(fetchAndRender, POLL_INTERVAL);
}

export function teardown(): void {
  if (_pollTimer !== null) {
    clearInterval(_pollTimer);
    _pollTimer = null;
  }
  _container = null;
}

async function fetchAndRender(): Promise<void> {
  try {
    const sessions = await fetchLobbySessions();
    renderList(sessions);
  } catch {
    // Leave the current list in place on transient fetch errors.
  }
}

function renderList(sessions: LobbySession[]): void {
  const listEl = _container?.querySelector('#lobby-list');
  if (!listEl) return;

  if (sessions.length === 0) {
    listEl.innerHTML = `
      <p class="lobby-empty" style="color:var(--color-text-muted)">
        No public sessions yet.
        <button class="btn-sm filter-btn" id="lobby-empty-create-btn" style="margin-left:8px">Create one!</button>
      </p>
    `;
    listEl.querySelector('#lobby-empty-create-btn')!.addEventListener('click', () => navigate('create'));
    return;
  }

  listEl.innerHTML = `
    <div class="session-list">
      ${sessions.map((s) => sessionRowHTML(s)).join('')}
    </div>
  `;
}

function sessionRowHTML(s: LobbySession): string {
  const writers = s.writerCount === 1 ? '1 writer' : `${s.writerCount} writers`;
  return `
    <div class="session-card">
      <div class="session-card-header">
        <span class="session-card-title">${escHtml(s.hostUsername)}</span>
        <span class="session-card-meta">${s.goal} words &middot; ${writers}</span>
      </div>
      <div class="session-card-actions">
        <button class="btn-primary btn-sm" data-join-id="${escAttr(s.id)}">Join</button>
      </div>
    </div>
  `;
}

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}

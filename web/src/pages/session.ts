import { navigate, setNavigationGuard, clearNavigationGuard } from '../nav';
import { saveSession, loadSession } from '../storage';
import { getStoredParticipant, setStoredParticipant, getLastUsername } from '../participant-store';
import { createSession } from '../api';
import { createWSManager } from '../ws';
import { countWords, fixQuotes } from '../utils';
import type { WSManager } from '../ws';
import type { ServerSession, ServerParticipant, ServerMessage } from '../types';

const USERNAME_RE = /^[a-zA-Z0-9_-]+$/;

type PageState = 'joining' | 'username-prompt' | 'lobby' | 'countdown' | 'active' | 'ended';

let _container: HTMLElement | null = null;
let _sessionId = '';
let _myParticipantId = '';
let _myUsername = '';
let _session: ServerSession | null = null;
let _pageState: PageState = 'joining';
let _writing = '';

let _wsManager: WSManager | null = null;
let _autosaveTimer: ReturnType<typeof setInterval> | null = null;
let _syncTimer: ReturnType<typeof setInterval> | null = null;
let _countdownTimer: ReturnType<typeof setInterval> | null = null;

export function mount(container: HTMLElement, sessionId: string): void {
  _container = container;
  _sessionId = sessionId;
  _myParticipantId = '';
  _myUsername = '';
  _session = null;
  _writing = '';

  const stored = getStoredParticipant(sessionId);

  if (stored) {
    // Known participant — connect and rejoin immediately.
    _myParticipantId = stored.participantId;
    _myUsername = stored.username;

    // Pre-load any locally saved writing.
    const record = loadSession(sessionId);
    if (record) _writing = record.writing;

    renderJoining();
    openWS();
  } else {
    // Unknown participant — ask for a username first.
    renderUsernamePrompt();
  }
}

export function teardown(): void {
  clearAllTimers();
  clearNavigationGuard();
  window.removeEventListener('beforeunload', handleBeforeUnload);

  if (_wsManager) {
    _wsManager.close();
    _wsManager = null;
  }

  // Best-effort writing save.
  if (_pageState === 'active' && _writing) {
    persistWriting();
  }

  _container = null;
  _sessionId = '';
  _myParticipantId = '';
  _myUsername = '';
  _session = null;
  _pageState = 'joining';
  _writing = '';
}

function openWS(): void {
  // Capture session ID so the permanent-close callback can detect if it fires
  // after teardown (which resets _sessionId to '') or after navigation to a
  // different session. Without this guard, the async WS close event fires on
  // the new page's module state and calls renderEnded() there.
  const sessionIdAtOpen = _sessionId;
  _wsManager = createWSManager(
    _sessionId,
    () =>
      _myParticipantId
        ? { type: 'rejoin', participantId: _myParticipantId }
        : { type: 'join', username: _myUsername },
    handleServerMessage,
    () => {
      if (_sessionId !== sessionIdAtOpen) return;
      if (_pageState !== 'ended' && _pageState !== 'username-prompt') renderEnded();
    },
  );
}

function handleServerMessage(msg: ServerMessage): void {
  switch (msg.type) {
    case 'session_state': {
      if (!msg.session || !msg.myParticipantId) return;
      _myParticipantId = msg.myParticipantId;
      _session = msg.session;

      // Persist identity so future page loads can rejoin.
      setStoredParticipant(_sessionId, {
        participantId: _myParticipantId,
        username: _myUsername || (participantById(_session, _myParticipantId)?.username ?? ''),
      });
      _myUsername = participantById(_session, _myParticipantId)?.username ?? _myUsername;

      if (_session.status === 'waiting') {
        renderLobby();
      } else if (_session.status === 'active') {
        // Joined mid-session or reconnecting — skip countdown.
        enterActive();
      } else {
        renderEnded();
      }
      break;
    }

    case 'participant_joined': {
      if (!msg.participant || !_session) return;
      _session = {
        ..._session,
        participants: [..._session.participants, msg.participant],
      };
      if (_pageState === 'lobby') updateLobbyParticipantList();
      break;
    }

    case 'participant_updated': {
      if (!msg.participant || !_session) return;
      _session = {
        ..._session,
        participants: _session.participants.map((p) =>
          p.id === msg.participant!.id ? msg.participant! : p,
        ),
      };
      if (_pageState === 'active') {
        updateParticipantItem(msg.participant);
        reorderParticipantList();
      }
      break;
    }

    case 'session_started':
      if (_pageState === 'lobby') startCountdown();
      break;

    case 'session_ended':
      endSession();
      break;

    case 'session_redirect':
      if (msg.nextSessionId) {
        setStoredParticipant(msg.nextSessionId, { participantId: '', username: _myUsername });
        navigate(`session/${msg.nextSessionId}`);
      }
      break;

    case 'error':
      if (_pageState === 'username-prompt' || _pageState === 'joining') {
        // Show error on the username form. Must happen before close() so that
        // _pageState is 'username-prompt' when onPermanentClose fires.
        showUsernameError(msg.message ?? 'Could not join session.');
        // Explicitly close to stop the auto-reconnect loop in ws.ts.
        _wsManager?.close();
      }
      break;
  }
}

function renderJoining(): void {
  _pageState = 'joining';
  setHTML(`
    <div class="joining-spinner">
      <div class="spinner"></div>
      <p>Connecting…</p>
    </div>
  `);
}

function renderUsernamePrompt(): void {
  _pageState = 'username-prompt';
  setHTML(`
    <h1>Join Session</h1>
    <p class="subtitle">Enter a username to join this writing session.</p>
    <form id="join-form" novalidate>
      <div class="form-group">
        <label for="username">Username</label>
        <input type="text" id="username" placeholder="your_name"
          autocomplete="off" maxlength="40" value="${escAttr(getLastUsername())}" />
        <div class="field-error" id="username-error"></div>
      </div>
      <div class="field-error" id="join-error"></div>
      <button type="submit" class="btn-primary" id="join-btn">Join</button>
    </form>
  `);

  const form = _container!.querySelector<HTMLFormElement>('#join-form')!;
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    const input = form.querySelector<HTMLInputElement>('#username')!;
    const err = form.querySelector<HTMLElement>('#username-error')!;
    const username = input.value.trim();

    if (!username) {
      err.textContent = 'Username is required.';
      return;
    }
    if (!USERNAME_RE.test(username)) {
      err.textContent = 'Only letters, numbers, underscores, and hyphens are allowed.';
      return;
    }
    err.textContent = '';
    _myUsername = username;

    const joinBtn = form.querySelector<HTMLButtonElement>('#join-btn')!;
    joinBtn.disabled = true;
    joinBtn.textContent = 'Joining…';

    renderJoining();
    openWS();
  });
}

function showUsernameError(message: string): void {
  renderUsernamePrompt();
  const err = _container?.querySelector<HTMLElement>('#join-error');
  if (err) err.textContent = message;
}

function renderLobby(): void {
  _pageState = 'lobby';
  const s = _session!;
  const isHost = isMyParticipantHost();
  const url = `${window.location.origin}${window.location.pathname}#session/${s.id}`;

  const visibility = s.public ? 'Public' : 'Private';

  setHTML(`
    <h1>Waiting to start</h1>
    <p class="subtitle">Goal: ${s.goal} words &middot; ${visibility}</p>

    <div class="lobby-url-wrap">
      <label class="lobby-url-label">Share this link with others</label>
      <div class="lobby-url-row">
        <input type="text" class="lobby-url-input" id="session-url" readonly value="${escAttr(url)}" />
        <button class="btn-sm filter-btn" id="copy-btn">Copy</button>
      </div>
    </div>

    <div id="participant-list-wrap">
      ${renderParticipantListHTML(s.participants)}
    </div>

    ${isHost
      ? '<button class="btn-primary" id="start-btn" style="margin-top:8px">Start</button>'
      : '<p class="lobby-waiting-msg">Waiting for the host to start…</p>'
    }
  `);

  _container!.querySelector('#copy-btn')?.addEventListener('click', () => {
    navigator.clipboard.writeText(url).then(() => {
      const btn = _container?.querySelector<HTMLButtonElement>('#copy-btn');
      if (btn) {
        btn.textContent = 'Copied!';
        setTimeout(() => { if (btn) btn.textContent = 'Copy'; }, 2000);
      }
    });
  });

  _container!.querySelector('#start-btn')?.addEventListener('click', () => {
    _wsManager?.send({ type: 'start' });
  });
}

function updateLobbyParticipantList(): void {
  if (!_session || !_container) return;
  const wrap = _container.querySelector('#participant-list-wrap');
  if (wrap) wrap.innerHTML = renderParticipantListHTML(_session.participants);
}

function startCountdown(): void {
  _pageState = 'countdown';
  let count = 3;
  setHTML(`
    <h1>Get ready…</h1>
    <p class="session-meta">Goal: ${_session!.goal} words</p>
    <div class="countdown-wrap">
      <div class="countdown-number" id="countdown-num">${count}</div>
      <div class="countdown-label">Starting soon</div>
    </div>
  `);

  _countdownTimer = setInterval(() => {
    count -= 1;
    if (count > 0) {
      const el = _container?.querySelector<HTMLElement>('#countdown-num');
      if (el) el.textContent = String(count);
    } else {
      clearInterval(_countdownTimer!);
      _countdownTimer = null;
      enterActive();
    }
  }, 1000);
}

function enterActive(): void {
  _pageState = 'active';
  const s = _session!;
  const sorted = sortParticipants(s.participants);

  setHTML(`
    <p class="session-meta">Goal: ${s.goal} words</p>
    <div class="participant-list" id="participant-list">
      ${sorted.map((p) => participantItemHTML(p)).join('')}
    </div>
    <textarea id="writing-area"></textarea>
    <div class="btn-row" style="margin-top:12px">
      <button class="btn-secondary btn-sm" id="fix-quotes-btn">Fix Quotes</button>
      <button class="btn-danger btn-sm" id="end-btn">End Session</button>
    </div>
  `);

  const textarea = _container!.querySelector<HTMLTextAreaElement>('#writing-area')!;
  textarea.value = _writing;
  textarea.addEventListener('input', onTextareaInput);
  textarea.focus();

  _container!.querySelector('#fix-quotes-btn')?.addEventListener('click', () => {
    const fixed = fixQuotes(_writing);
    if (fixed === _writing) return;
    textarea.value = fixed;
    textarea.dispatchEvent(new Event('input'));
  });

  _container!.querySelector('#end-btn')?.addEventListener('click', () => {
    if (confirm('End the session for everyone?')) {
      _wsManager?.send({ type: 'end' });
    }
  });

  // 1-second word count sync to server.
  _syncTimer = setInterval(() => {
    _wsManager?.send({ type: 'update', wordCount: countWords(_writing) });
  }, 1000);

  // 5-second autosave to localStorage.
  _autosaveTimer = setInterval(persistWriting, 5000);

  setNavigationGuard(() =>
    confirm('Your session is in progress. Your writing will be saved. Leave anyway?')
  );
  window.addEventListener('beforeunload', handleBeforeUnload);
}

function renderEnded(): void {
  _pageState = 'ended';
  clearAllTimers();
  clearNavigationGuard();
  window.removeEventListener('beforeunload', handleBeforeUnload);

  persistWriting();

  const s = _session;

  const ranked = s
    ? [...s.participants]
        .filter((p) => p.completed)
        .sort((a, b) => a.finishOrder - b.finishOrder)
    : [];

  const rankBadge = ['🥇', '🥈', '🥉'];

  setHTML(`
    ${ranked.length > 0 ? `
      <h2 style="margin-bottom:12px">Final Rankings</h2>
      <div class="session-list">
        ${ranked.map((p, i) => `
          <div class="session-card">
            <div class="session-card-header">
              <span class="session-card-title">
                ${rankBadge[i] ?? `#${i + 1}`} ${escHtml(p.username)}
                ${p.id === _myParticipantId ? '<span class="you-badge">(you)</span>' : ''}
              </span>
              <span class="session-card-meta">${p.wordCount} words</span>
            </div>
          </div>
        `).join('')}
      </div>
    ` : ''}

    ${_writing ? `
      <div class="ended-writing-wrap">
        <div class="btn-row" style="margin-bottom:8px">
          <button class="btn-secondary btn-sm" id="fix-quotes-ended-btn">Fix Quotes</button>
        </div>
        <textarea id="ended-writing-area">${escHtml(_writing)}</textarea>
      </div>
    ` : ''}

    <div class="btn-row" style="margin-top:28px">
      <button class="btn-primary" id="start-again-btn">
        Start Again (${s?.goal ?? '?'} words)
      </button>
      <button class="btn-secondary" id="change-goal-btn">Change Goal</button>
    </div>
  `);

  const endedTextarea = _container!.querySelector<HTMLTextAreaElement>('#ended-writing-area');
  if (endedTextarea) {
    endedTextarea.focus();
    // Place cursor at end.
    endedTextarea.selectionStart = endedTextarea.selectionEnd = endedTextarea.value.length;
    endedTextarea.addEventListener('input', (e) => {
      _writing = (e.target as HTMLTextAreaElement).value;
      persistWriting();
    });
  }

  _container!.querySelector('#fix-quotes-ended-btn')?.addEventListener('click', () => {
    if (!endedTextarea) return;
    const fixed = fixQuotes(_writing);
    if (fixed === _writing) return;
    endedTextarea.value = fixed;
    _writing = fixed;
    persistWriting();
  });

  _container!.querySelector('#start-again-btn')?.addEventListener('click', () => {
    const goal = s?.goal;
    if (!goal) { navigate('create'); return; }
    createSession({ goal, username: _myUsername })
      .then(({ id, participantId }) => {
        setStoredParticipant(id, { participantId, username: _myUsername });
        // Notify other participants still on the ended session to follow.
        fetch(`/api/sessions/${_sessionId}/redirect`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ nextSessionId: id }),
        }).catch(() => {/* best-effort */});
        navigate(`session/${id}`);
      })
      .catch(() => navigate('create'));
  });

  _container!.querySelector('#change-goal-btn')?.addEventListener('click', () => {
    navigate('create');
  });
}

function endSession(): void {
  const wasInLobby = _pageState === 'lobby';
  if (_pageState === 'active') persistWriting();
  if (_session) _session = { ..._session, status: 'ended' };
  if (wasInLobby) {
    renderHostLeft();
  } else {
    renderEnded();
  }
}

function renderHostLeft(): void {
  _pageState = 'ended';
  clearAllTimers();
  clearNavigationGuard();
  window.removeEventListener('beforeunload', handleBeforeUnload);
  setHTML(`
    <h1>Session Ended</h1>
    <p style="margin-bottom:20px">The host left. This session has ended.</p>
    <button class="btn-secondary" id="back-to-lobby-btn">Back to Lobby</button>
  `);
  _container!.querySelector('#back-to-lobby-btn')!.addEventListener('click', () => navigate('lobby'));
}

function renderParticipantListHTML(participants: ServerParticipant[]): string {
  const sorted = sortParticipants(participants);
  return `
    <div class="participant-list" id="participant-list">
      ${sorted.map((p) => participantItemHTML(p)).join('')}
    </div>
  `;
}

function participantItemHTML(p: ServerParticipant): string {
  const isMe = p.id === _myParticipantId;
  const goal = _session?.goal ?? 1;
  const pct = Math.min(100, Math.round((p.wordCount / goal) * 100));
  const classes = [
    'participant-item',
    isMe ? 'participant-item--self' : '',
    !p.connected ? 'participant-item--disconnected' : '',
    p.completed ? 'participant-item--completed' : '',
  ].filter(Boolean).join(' ');

  const rank = p.finishOrder > 0
    ? `<span class="participant-rank">${['🥇', '🥈', '🥉'][p.finishOrder - 1] ?? `#${p.finishOrder}`}</span>`
    : '<span class="participant-rank"></span>';

  return `
    <div class="${classes}" data-pid="${p.id}">
      ${rank}
      <div class="participant-info">
        <div class="participant-name">
          ${escHtml(p.username)}${isMe ? ' <span class="you-badge">(you)</span>' : ''}
          ${!p.connected ? ' <span class="disconnected-badge">disconnected</span>' : ''}
        </div>
        <div class="progress-track" style="margin-top:4px">
          <div class="progress-fill${p.completed ? ' complete' : ''}" style="width:${pct}%"></div>
        </div>
      </div>
      <span class="participant-wc">${p.wordCount}/${goal}</span>
    </div>
  `;
}

/** Update one participant's item in place without rebuilding the list. */
function updateParticipantItem(p: ServerParticipant): void {
  if (!_container) return;
  const existing = _container.querySelector<HTMLElement>(`[data-pid="${p.id}"]`);
  if (!existing) {
    // New participant joined mid-session — insert without disturbing textarea.
    const list = _container.querySelector('#participant-list');
    if (list) {
      const div = document.createElement('div');
      div.innerHTML = participantItemHTML(p).trim();
      list.appendChild(div.firstElementChild!);
    }
    return;
  }
  // Replace the existing element in place.
  const next = document.createElement('div');
  next.innerHTML = participantItemHTML(p).trim();
  existing.replaceWith(next.firstElementChild!);
}

/** Re-sort participant items in the DOM: finishers first, then by joinOrder. */
function reorderParticipantList(): void {
  if (!_session || !_container) return;
  const list = _container.querySelector('#participant-list');
  if (!list) return;

  const sorted = sortParticipants(_session.participants);
  for (const p of sorted) {
    const el = _container.querySelector(`[data-pid="${p.id}"]`);
    if (el) list.appendChild(el); // move to end in sorted order
  }
}

/** Sort: finishers ascending by finishOrder, then non-finishers by joinOrder. */
function sortParticipants(participants: ServerParticipant[]): ServerParticipant[] {
  return [...participants].sort((a, b) => {
    const aFinished = a.finishOrder > 0;
    const bFinished = b.finishOrder > 0;
    if (aFinished && bFinished) return a.finishOrder - b.finishOrder;
    if (aFinished) return -1;
    if (bFinished) return 1;
    return a.joinOrder - b.joinOrder;
  });
}

function onTextareaInput(e: Event): void {
  _writing = (e.target as HTMLTextAreaElement).value;

  const wc = countWords(_writing);

  // Update the user's own row in the leaderboard immediately (the server
  // broadcasts updates to everyone *except* the sender, so we do it locally).
  if (_session && _myParticipantId) {
    _session = {
      ..._session,
      participants: _session.participants.map((p) =>
        p.id === _myParticipantId ? { ...p, wordCount: wc } : p,
      ),
    };
    const me = _session.participants.find((p) => p.id === _myParticipantId);
    if (me) updateParticipantItem(me);
  }
}

function handleBeforeUnload(): void {
  persistWriting();
}

function persistWriting(): void {
  if (!_sessionId || !_myUsername) return;
  const existing = loadSession(_sessionId);
  saveSession({
    id: _sessionId,
    username: _myUsername,
    goal: _session?.goal ?? 0,
    writing: _writing,
    createdAt: existing?.createdAt ?? Date.now(),
  });
}

function clearAllTimers(): void {
  [_autosaveTimer, _syncTimer, _countdownTimer].forEach((t) => {
    if (t !== null) clearInterval(t);
  });
  _autosaveTimer = null;
  _syncTimer = null;
  _countdownTimer = null;
}

function isMyParticipantHost(): boolean {
  if (!_session) return false;
  const me = participantById(_session, _myParticipantId);
  return me?.joinOrder === 1;
}

function participantById(session: ServerSession, id: string): ServerParticipant | undefined {
  return session.participants.find((p) => p.id === id);
}

function setHTML(html: string): void {
  if (_container) _container.innerHTML = html;
}

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}

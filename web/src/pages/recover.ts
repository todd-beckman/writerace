import { clearAllSessions, deleteSession, listSessions, saveSession } from '../storage';
import { countWords, fixQuotes, formatDate, escHtml } from '../utils';

const PAGE_SIZE = 10;

type Filter = 'all' | 'complete' | 'incomplete';

interface State {
  filter: Filter;
  page: number;
}

export function mount(container: HTMLElement): void {
  render(container, { filter: 'all', page: 0 });
}

export function teardown(): void {
  // No timers or global listeners to clean up.
}

function render(container: HTMLElement, state: State): void {
  const { filter, page } = state;

  const allSessions = listSessions()
    .slice()
    .sort((a, b) => b.createdAt - a.createdAt);

  const filtered = allSessions.filter((s) => {
    if (filter === 'complete') return !!s.completedAt;
    if (filter === 'incomplete') return !s.completedAt;
    return true;
  });

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const clampedPage = Math.min(page, totalPages - 1);
  const pageSessions = filtered.slice(clampedPage * PAGE_SIZE, (clampedPage + 1) * PAGE_SIZE);

  const filterBar = `
    <div class="filter-bar">
      <button class="btn-sm filter-btn${filter === 'all' ? ' active' : ''}" data-filter="all">All</button>
      <button class="btn-sm filter-btn${filter === 'complete' ? ' active' : ''}" data-filter="complete">Complete</button>
      <button class="btn-sm filter-btn${filter === 'incomplete' ? ' active' : ''}" data-filter="incomplete">Incomplete</button>
    </div>
  `;

  let bodyHtml: string;

  if (filtered.length === 0) {
    bodyHtml = `
      <div class="empty-state">
        <p>No sessions found.</p>
        <p>Complete a writing session to see it here.</p>
      </div>
    `;
  } else {
    const cards = pageSessions
      .map((s) => {
        const wc = countWords(s.writing);
        const status = s.completedAt ? 'Completed' : 'Incomplete';
        const date = formatDate(s.createdAt);
        const hasWriting = s.writing.trim().length > 0;
        const bodyClass = hasWriting ? '' : ' empty';
        const bodyText = hasWriting ? escHtml(s.writing) : 'No writing saved.';

        return `
          <div class="session-card" data-session-id="${escHtml(s.id)}">
            <div class="session-card-header">
              <span class="session-card-title">${escHtml(s.username)}</span>
              <span class="session-card-meta">${wc} / ${s.goal} words &bull; ${status} &bull; ${date}</span>
            </div>
            <div class="session-card-body${bodyClass}">${bodyText}</div>
            <div class="session-card-actions">
              ${hasWriting ? '<button class="btn-sm copy-writing-btn">Copy</button>' : ''}
              ${hasWriting ? '<button class="btn-sm filter-btn fix-quotes-btn">Fix Quotes</button>' : ''}
              <button class="btn-danger btn-sm delete-session-btn">Delete</button>
            </div>
          </div>
        `;
      })
      .join('');

    const pagination = totalPages > 1 ? `
      <div class="pagination">
        <button class="btn-sm pagination-btn" id="prev-btn" ${clampedPage === 0 ? 'disabled' : ''}>Previous</button>
        <span class="pagination-info">Page ${clampedPage + 1} of ${totalPages}</span>
        <button class="btn-sm pagination-btn" id="next-btn" ${clampedPage >= totalPages - 1 ? 'disabled' : ''}>Next</button>
      </div>
    ` : '';

    bodyHtml = `
      <div class="session-list">${cards}</div>
      ${pagination}
      <div class="danger-zone">
        <h2>Danger Zone</h2>
        <button class="btn-danger" id="clear-btn">Clear All Sessions</button>
      </div>
    `;
  }

  container.innerHTML = `
    <h1>Recovered Writing</h1>
    <p class="subtitle">Your writing from previous sessions. Nothing here is sent to the server — it lives only in your browser. <a href="#privacy" class="link">Privacy policy</a></p>
    ${filterBar}
    ${bodyHtml}
  `;

  container.querySelectorAll<HTMLButtonElement>('.filter-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      render(container, { filter: btn.dataset.filter as Filter, page: 0 });
    });
  });

  container.querySelectorAll<HTMLButtonElement>('.copy-writing-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      const card = btn.closest<HTMLElement>('[data-session-id]');
      const sessionId = card?.dataset.sessionId;
      if (!sessionId) return;
      const record = allSessions.find((s) => s.id === sessionId);
      if (!record) return;
      const markCopied = () => {
        btn.textContent = 'Copied!';
        setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
      };
      if (navigator.clipboard) {
        navigator.clipboard.writeText(record.writing).then(markCopied);
      } else {
        const input = document.createElement('input');
        input.value = record.writing;
        document.body.appendChild(input);
        input.select();
        document.execCommand('copy');
        document.body.removeChild(input);
        markCopied();
      }
    });
  });

  container.querySelectorAll<HTMLButtonElement>('.fix-quotes-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      const card = btn.closest<HTMLElement>('[data-session-id]');
      const sessionId = card?.dataset.sessionId;
      if (!sessionId) return;
      const record = allSessions.find((s) => s.id === sessionId);
      if (!record) return;
      const fixed = fixQuotes(record.writing);
      if (fixed === record.writing) return;
      saveSession({ ...record, writing: fixed });
      render(container, { filter, page: clampedPage });
    });
  });

  container.querySelectorAll<HTMLButtonElement>('.delete-session-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      const card = btn.closest<HTMLElement>('[data-session-id]');
      const sessionId = card?.dataset.sessionId;
      if (!sessionId) return;
      if (confirm('Delete this session? This cannot be undone.')) {
        deleteSession(sessionId);
        render(container, { filter, page: clampedPage });
      }
    });
  });

  container.querySelector('#prev-btn')?.addEventListener('click', () => {
    render(container, { filter, page: clampedPage - 1 });
  });

  container.querySelector('#next-btn')?.addEventListener('click', () => {
    render(container, { filter, page: clampedPage + 1 });
  });

  container.querySelector('#clear-btn')?.addEventListener('click', () => {
    if (
      confirm(
        'This will permanently delete all saved writing. This cannot be undone. Continue?'
      )
    ) {
      clearAllSessions();
      render(container, { filter: 'all', page: 0 });
    }
  });
}


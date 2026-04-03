import './style.css';
import { navigate } from './nav';
import { initRouter } from './router';

const THEME_KEY = 'writerace:theme';
type Theme = 'light' | 'dark';

function getSystemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getTheme(): Theme {
  const stored = localStorage.getItem(THEME_KEY);
  if (stored === 'light' || stored === 'dark') return stored;
  return getSystemTheme();
}

function applyTheme(theme: Theme, animate = false): void {
  if (animate) {
    document.documentElement.classList.add('theme-switching');
    setTimeout(() => document.documentElement.classList.remove('theme-switching'), 400);
  }
  document.documentElement.classList.toggle('dark', theme === 'dark');
  const btn = document.getElementById('nav-theme');
  if (btn) btn.textContent = theme === 'dark' ? '☀ Light' : '☾ Dark';
}

function toggleTheme(): void {
  const current = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
  const next: Theme = current === 'dark' ? 'light' : 'dark';
  localStorage.setItem(THEME_KEY, next);
  applyTheme(next, true);
}

// Apply theme before rendering anything to avoid a flash.
applyTheme(getTheme());

// Track system theme changes; only follow them if the user has no manual override.
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  if (!localStorage.getItem(THEME_KEY)) {
    applyTheme(getSystemTheme());
  }
});

const root = document.getElementById('root')!;

root.innerHTML = `
  <nav>
    <span class="nav-brand" id="nav-home">Write Race</span>
    <div class="nav-links">
      <span class="nav-link" id="nav-lobby">Lobby</span>
      <span class="nav-link" id="nav-new">Create</span>
      <span class="nav-link" id="nav-recover">Recover</span>
      <span class="nav-link" id="nav-privacy">Privacy</span>
      <button class="nav-theme-btn" id="nav-theme" aria-label="Toggle theme"></button>
    </div>
  </nav>
  <main>
    <div id="page-content"></div>
  </main>
`;

// Set the initial theme label now that the button exists in the DOM.
applyTheme(getTheme());

root.querySelector('#nav-home')!.addEventListener('click', () => navigate('lobby'));
root.querySelector('#nav-lobby')!.addEventListener('click', () => navigate('lobby'));
root.querySelector('#nav-new')!.addEventListener('click', () => navigate('create'));
root.querySelector('#nav-recover')!.addEventListener('click', () => navigate('recover'));
root.querySelector('#nav-privacy')!.addEventListener('click', () => navigate('privacy'));
root.querySelector('#nav-theme')!.addEventListener('click', toggleTheme);

initRouter();

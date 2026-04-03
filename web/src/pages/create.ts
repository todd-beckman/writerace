import { navigate } from '../nav';
import { createSession } from '../api';
import { setStoredParticipant, getLastUsername } from '../participant-store';
import { escAttr, validateUsername } from '../utils';

let _teardownFn: (() => void) | null = null;

export function mount(container: HTMLElement, params: URLSearchParams): void {
  const prefillUsername = params.get('username') || getLastUsername();
  const prefillGoal = params.get('goal') ?? '';

  container.innerHTML = `
    <h1>New Session</h1>
    <p class="subtitle">Set a goal and start writing.</p>

    <form id="create-form" novalidate>
      <div class="form-group">
        <label for="username">Username</label>
        <input
          type="text"
          id="username"
          placeholder="your_name"
          autocomplete="off"
          maxlength="40"
          value="${escAttr(prefillUsername)}"
        />
        <div class="field-error" id="username-error"></div>
      </div>

      <div class="form-group">
        <label for="goal">Word count goal</label>
        <input
          type="number"
          id="goal"
          placeholder="500"
          min="1"
          max="1000000"
          value="${escAttr(prefillGoal)}"
        />
        <div class="field-error" id="goal-error"></div>
      </div>

      <div class="form-group">
        <label class="checkbox-label">
          <input type="checkbox" id="public" />
          Make session public
        </label>
      </div>

      <div class="field-error" id="server-error"></div>

      <button type="submit" class="btn-primary" id="submit-btn">Start Session</button>
    </form>
  `;

  const form = container.querySelector<HTMLFormElement>('#create-form')!;
  const usernameInput = form.querySelector<HTMLInputElement>('#username')!;
  const goalInput = form.querySelector<HTMLInputElement>('#goal')!;
  const usernameError = form.querySelector<HTMLElement>('#username-error')!;
  const goalError = form.querySelector<HTMLElement>('#goal-error')!;
  const serverError = form.querySelector<HTMLElement>('#server-error')!;
  const publicInput = form.querySelector<HTMLInputElement>('#public')!;
  const submitBtn = form.querySelector<HTMLButtonElement>('#submit-btn')!;

  async function onSubmit(e: Event): Promise<void> {
    e.preventDefault();
    serverError.textContent = '';

    let valid = true;

    const username = usernameInput.value.trim();
    const usernameErr = validateUsername(username);
    if (usernameErr) {
      usernameError.textContent = usernameErr;
      valid = false;
    } else {
      usernameError.textContent = '';
    }

    const goalRaw = goalInput.value;
    const goal = parseInt(goalRaw, 10);
    if (!goalRaw || isNaN(goal) || goal < 1) {
      goalError.textContent = 'Enter a word count of at least 1.';
      valid = false;
    } else {
      goalError.textContent = '';
    }

    if (!valid) return;

    submitBtn.disabled = true;
    submitBtn.textContent = 'Creating…';

    try {
      const { id, participantId } = await createSession({ goal, username, public: publicInput.checked });
      setStoredParticipant(id, { participantId, username });
      navigate(`session/${id}`);
    } catch (err) {
      serverError.textContent =
        err instanceof Error ? err.message : 'Could not create session. Try again.';
      submitBtn.disabled = false;
      submitBtn.textContent = 'Start Session';
    }
  }

  form.addEventListener('submit', onSubmit);
  _teardownFn = () => form.removeEventListener('submit', onSubmit);
}

export function teardown(): void {
  _teardownFn?.();
  _teardownFn = null;
}


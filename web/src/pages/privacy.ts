export function mount(container: HTMLElement): void {
    render(container);
}

export function teardown(): void {
    // No timers or global listeners to clean up.
}

function render(container: HTMLElement): void {
    container.innerHTML = `
    <h1>Privacy</h1>
    <p class="subtitle">What data Write Race collects and where it goes.</p>

    <h2>Data sent to the server</h2>
    <p>The following data is transmitted to the server during a session:</p>
    <ul>
      <li><strong>Username</strong> — the name you enter when joining a session.</li>
      <li><strong>Word count goal</strong> — the target word count set when the session is created.</li>
      <li><strong>Word count</strong> — the number of words in your entry, sent once per second while a session is active.</li>
    </ul>
    <p>
      This data is held in memory only for the duration of the session.
      The server has no database.
      Once a session ends, its data is gone and cannot be recovered from the server.
    </p>

    <h2>Data that stays on your device</h2>
    <p>
      The text you type is <strong>never sent to the server</strong>.
      It does not leave your device.
      Write Race saves it to your browser's local storage as a backup so you can review it later on the
      <span class="inline-link" id="privacy-recover-link">Recover</span> page.
      You can delete all saved writing from that page at any time.
    </p>
    <p>Write Race also stores the following in your browser's local storage:</p>
    <ul>
      <li><strong>Session writing</strong> — the text you typed, your username, word count, and timestamps for each session.</li>
      <li><strong>Participant identity</strong> — a participant ID and username per session, used to rejoin a session if your connection drops.</li>
      <li><strong>Last username</strong> — the most recent username you used, pre-filled for convenience when creating or joining a session.</li>
      <li><strong>Theme preference</strong> — whether you have chosen light or dark mode.</li>
    </ul>
    <p>None of this local storage data is transmitted anywhere.</p>
  `;

    container.querySelector('#privacy-recover-link')?.addEventListener('click', () => {
        window.location.hash = 'recover';
    });
}

# Write Race

Write Race is a browser-based competitive writing application where users race to reach a shared word count goal. Writers join a session, wait for the host to start, and then type as fast as they can — each participant's progress is visible to everyone in real time.

## Features

### Frontend

- **Lobby** — The home page shows all public sessions currently waiting for players. Each listing displays the host's name, word count goal, and current writer count. Clicking "Join" navigates directly to that session.
- **Create Session** — Set a username, a word count goal, and whether the session is public or private. Creating a session generates a shareable URL.
- **Writing Session** — A countdown starts when the host presses Start. Once active, a progress bar above the textarea tracks word count against the goal. When the goal is reached, the bar changes color and a win indicator appears. Other participants' progress bars update live.
- **Completion leaderboard** — As each writer finishes, their progress bar moves to the top of the list in ranked order (1st, 2nd, 3rd…).
- **Disconnect handling** — A disconnected participant's progress bar turns gray. They can rejoin by returning to the same URL and their word count is restored.
- **Recover Writing** — A dedicated page reads localStorage and lists all previous sessions with their saved writing. A clearly labeled button clears all saved data.

### Backend

- **Session management** — Sessions are created with a unique short ID, a word count goal, a public/private flag, and a one-hour timeout. Up to 5 participants may join a session.
- **Real-time sync via WebSockets** — Word counts are pushed to the server every second and broadcast to all participants in the session. No writing content is ever transmitted — only integer word counts.
- **Lobby endpoint** — `GET /api/sessions` returns all public sessions in waiting status that are not yet at capacity.
- **Host rules** — If the host disconnects before the session starts, the session ends immediately and remaining participants are notified. Once a session is active, host disconnection is treated the same as any other participant disconnection.
- **Session end conditions** — A session ends when all writers reach the goal, the host manually ends it, or the one-hour timeout is reached.
- **In-memory storage** — All session state is held in memory, designed so a database layer can be added later without restructuring the API.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and project structure.

import type { ClientMessage, ServerMessage } from './types';

export interface WSManager {
  send(msg: ClientMessage): void;
  close(): void;
}

/**
 * Opens a WebSocket to /ws/{sessionId}.
 *
 * getFirstMessage is called on every connect attempt and must return the
 * join or rejoin message to send immediately after opening. After the first
 * successful session_state, the caller should update whatever state
 * getFirstMessage reads so subsequent reconnects send rejoin.
 *
 * onMessage is called for every incoming server message.
 * onPermanentClose is called when the connection is closed intentionally
 * (via WSManager.close()) or when the session has ended.
 */
export function createWSManager(
  sessionId: string,
  getFirstMessage: () => ClientMessage,
  onMessage: (msg: ServerMessage) => void,
  onPermanentClose: () => void,
): WSManager {
  let ws: WebSocket | null = null;
  let intentionalClose = false;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  function buildURL(): string {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${window.location.host}/ws/${sessionId}`;
  }

  function connect(): void {
    ws = new WebSocket(buildURL());

    ws.addEventListener('open', () => {
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      ws!.send(JSON.stringify(getFirstMessage()));
    });

    ws.addEventListener('message', (event) => {
      try {
        const msg: ServerMessage = JSON.parse(event.data as string);
        onMessage(msg);
        // Once the session ends the server will close the connection;
        // treat that as a permanent close.
        if (msg.type === 'session_ended') {
          intentionalClose = true;
        }
      } catch {
        // Malformed JSON — ignore.
      }
    });

    ws.addEventListener('close', () => {
      ws = null;
      if (intentionalClose) {
        onPermanentClose();
        return;
      }
      // Unexpected disconnect — reconnect after a short delay.
      reconnectTimer = setTimeout(connect, 2000);
    });

    ws.addEventListener('error', () => {
      // The close event will fire next and trigger reconnect.
    });
  }

  connect();

  return {
    send(msg: ClientMessage): void {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(msg));
      }
    },
    close(): void {
      intentionalClose = true;
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      ws?.close();
    },
  };
}

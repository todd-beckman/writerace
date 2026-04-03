import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { createWSManager } from './ws';
import type { ClientMessage, ServerMessage } from './types';

type WSEventType = 'open' | 'message' | 'close' | 'error';

class MockWebSocket {
  static OPEN = 1;
  readyState = MockWebSocket.OPEN;
  url: string;
  private listeners: Partial<Record<WSEventType, EventListener>> = {};

  constructor(url: string) {
    this.url = url;
    instances.push(this);
  }

  addEventListener(type: WSEventType, listener: EventListener): void {
    this.listeners[type] = listener;
  }

  send = vi.fn();
  close = vi.fn().mockImplementation(() => {
    this.readyState = 3; // CLOSED
  });

  emit(type: WSEventType, event: unknown = {}): void {
    this.listeners[type]?.(event as Event);
  }
}

let instances: MockWebSocket[] = [];

beforeEach(() => {
  instances = [];
  vi.stubGlobal('WebSocket', MockWebSocket);
  vi.stubGlobal('window', {
    location: { protocol: 'http:', host: 'localhost' },
  });
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

function latestSocket(): MockWebSocket {
  return instances[instances.length - 1];
}

describe('createWSManager — connection lifecycle', () => {
  it('opens a WebSocket to the expected URL', () => {
    createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    expect(latestSocket().url).toBe('ws://localhost/ws/s1');
  });

  it('uses wss when protocol is https', () => {
    window.location.protocol = 'https:';
    createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    expect(latestSocket().url).toBe('wss://localhost/ws/s1');
  });

  it('sends the first message from getFirstMessage on open', () => {
    const firstMessage: ClientMessage = { type: 'join', username: 'alice' };
    createWSManager('s1', () => firstMessage, vi.fn(), vi.fn());
    latestSocket().emit('open');
    expect(latestSocket().send).toHaveBeenCalledWith(JSON.stringify(firstMessage));
  });

  it('calls onMessage with the parsed object on a valid JSON message', () => {
    const onMessage = vi.fn();
    createWSManager('s1', () => ({ type: 'join' }), onMessage, vi.fn());
    const msg: ServerMessage = { type: 'pong' };
    latestSocket().emit('message', { data: JSON.stringify(msg) });
    expect(onMessage).toHaveBeenCalledWith(msg);
  });

  it('does not throw on malformed JSON', () => {
    createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    expect(() => latestSocket().emit('message', { data: 'not json' })).not.toThrow();
  });
});

describe('createWSManager — reconnect behavior', () => {
  it('reconnects after ~2 seconds on unexpected close', () => {
    createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    expect(instances).toHaveLength(1);
    latestSocket().emit('close');
    vi.advanceTimersByTime(2000);
    expect(instances).toHaveLength(2);
  });

  it('does not reconnect on intentional close via manager.close()', () => {
    const manager = createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    manager.close();
    latestSocket().emit('close');
    vi.advanceTimersByTime(2000);
    expect(instances).toHaveLength(1);
  });

  it('calls onPermanentClose when session_ended message is followed by close', () => {
    const onPermanentClose = vi.fn();
    createWSManager('s1', () => ({ type: 'join' }), vi.fn(), onPermanentClose);
    latestSocket().emit('message', { data: JSON.stringify({ type: 'session_ended' }) });
    latestSocket().emit('close');
    expect(onPermanentClose).toHaveBeenCalledOnce();
    vi.advanceTimersByTime(2000);
    expect(instances).toHaveLength(1); // no reconnect
  });
});

describe('WSManager.send', () => {
  it('sends a JSON-serialized message when the socket is open', () => {
    const manager = createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    latestSocket().readyState = MockWebSocket.OPEN;
    const msg: ClientMessage = { type: 'ping' };
    manager.send(msg);
    expect(latestSocket().send).toHaveBeenCalledWith(JSON.stringify(msg));
  });

  it('does nothing when the socket is not open', () => {
    const manager = createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    latestSocket().readyState = 3; // CLOSED
    manager.send({ type: 'ping' });
    expect(latestSocket().send).not.toHaveBeenCalled();
  });
});

describe('WSManager.close', () => {
  it('closes the underlying WebSocket', () => {
    const manager = createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    manager.close();
    expect(latestSocket().close).toHaveBeenCalled();
  });

  it('cancels any pending reconnect timer', () => {
    const manager = createWSManager('s1', () => ({ type: 'join' }), vi.fn(), vi.fn());
    latestSocket().emit('close'); // triggers reconnect timer
    manager.close();
    vi.advanceTimersByTime(2000);
    expect(instances).toHaveLength(1); // no new socket created
  });
});

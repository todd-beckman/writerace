export interface SessionRecord {
  id: string;
  username: string;
  goal: number;
  writing: string;
  createdAt: number;
  completedAt?: number;
}

export type ServerSessionStatus = 'waiting' | 'active' | 'ended';

export interface ServerParticipant {
  id: string;
  username: string;
  wordCount: number;
  connected: boolean;
  completed: boolean;
  joinOrder: number;
  finishOrder: number; // 0 = not finished
}

export interface ServerSession {
  id: string;
  goal: number;
  public?: boolean;
  status: ServerSessionStatus;
  participants: ServerParticipant[];
}

export interface LobbySession {
  id: string;
  goal: number;
  hostUsername: string;
  writerCount: number;
}

// Client → server
export interface ClientMessage {
  type: 'join' | 'rejoin' | 'start' | 'update' | 'end' | 'ping';
  username?: string;
  participantId?: string;
  wordCount?: number;
}

// Server → client
export interface ServerMessage {
  type:
    | 'session_state'
    | 'participant_joined'
    | 'participant_updated'
    | 'session_started'
    | 'session_ended'
    | 'session_redirect'
    | 'error'
    | 'pong';
  session?: ServerSession;
  myParticipantId?: string;
  participant?: ServerParticipant;
  message?: string;
  nextSessionId?: string;
}

// POST /api/sessions
export interface CreateSessionRequest {
  goal: number;
  username: string;
  public?: boolean;
}

export interface CreateSessionResponse {
  id: string;
  participantId: string;
}

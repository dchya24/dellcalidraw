// WebSocket Message Types
export interface WSMessage<T = any> {
  type: string;
  payload: T;
}

// Element Types
export interface Element {
  id: string;
  type: string;
  x: number;
  y: number;
  width?: number;
  height?: number;
  angle?: number;
  stroke?: string;
  background?: string;
  fill?: string;
  data?: Record<string, any>;
}

// User Types
export interface User {
  id: string;
  username: string;
  color: string;
  connId?: string;
  joinedAt?: string;
  lastSeen?: string;
  isIdle?: boolean;
}

export interface Participant {
  id: string;
  username: string;
  color: string;
}

// Room Types
export interface RoomState {
  elements: Element[];
  participants: Participant[];
}

export interface CursorPosition {
  x: number;
  y: number;
}

export interface RemoteCursor {
  userId: string;
  username: string;
  color: string;
  position: CursorPosition;
  timestamp: number;
}

// Client → Server Payloads
export interface JoinRoomPayload {
  roomId: string;
  username: string;
}

export interface LeaveRoomPayload {
  roomId: string;
}

export interface UpdateElementsPayload {
  roomId: string;
  changes: ElementChanges;
}

export interface ElementChanges {
  added?: Element[];
  updated?: Element[];
  deleted?: string[];
}

export interface CursorMovePayload {
  roomId: string;
  position: CursorPosition;
}

// Server → Client Payloads
export interface RoomStatePayload {
  elements: Element[];
  participants: Participant[];
}

export interface UserJoinedPayload {
  userId: string;
  username: string;
  color: string;
}

export interface UserLeftPayload {
  userId: string;
}

export interface ElementsUpdatedPayload {
  userId: string;
  changes: ElementChanges;
}

export interface CursorUpdatedPayload {
  userId: string;
  username: string;
  color: string;
  position: CursorPosition;
}

export interface ErrorPayload {
  message: string;
  code?: string;
}

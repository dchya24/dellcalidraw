// WebSocket Message Types
export interface WSMessage<T = unknown> {
  type: string;
  payload: T;
}

// WebSocket Message Types
export interface WSMessage<T = unknown> {
  type: string;
  payload: T;
}

// Element Types - Matches backend ElementPayload
// Renamed from Element to ExcalidrawElementPayload to avoid DOM Element conflict
export interface ExcalidrawElementPayload {
  id: string;
  type: string;
  x: number;
  y: number;
  width?: number;
  height?: number;
  angle?: number;
  strokeColor?: string;
  backgroundColor?: string;
  fillStyle?: string;
  strokeWidth?: number;
  strokeStyle?: string;
  roughness?: number;
  opacity?: number;
  seed?: number;
  version?: number;
  versionNonce?: number;
  isDeleted?: boolean;
  groupIds?: string[];
  frameId?: string | null;
  boundElements?: BoundElementPayload[];
  updated?: number;
  link?: string | null;
  locked?: boolean;
  data?: Record<string, unknown>;
}

// Keep Element as alias for backward compatibility
/** @deprecated Use ExcalidrawElementPayload instead */
export type Element = ExcalidrawElementPayload;

export interface BoundElementPayload {
  id: string;
  type: "arrow" | "text";
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

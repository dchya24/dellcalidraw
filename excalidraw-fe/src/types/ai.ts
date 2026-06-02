import type { OrderedExcalidrawElement } from "@excalidraw/excalidraw/element/types";

// ─── Chat Messages ──────────────────────────────────────────────────────────

export type MessageRole = "user" | "assistant" | "system" | "tool";

export interface ChatMessage {
  id: string;
  role: MessageRole;
  content: string;
  timestamp: number;
  toolCalls?: ToolCall[];
  toolResults?: ToolResult[];
  /** Element IDs created by AI tool calls in this message */
  createdElementIds?: string[];
  /** Token usage reported by the LLM for this exchange */
  usage?: TokenUsage;
}

export interface TokenUsage {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: Record<string, unknown>;
}

export interface ToolResult {
  callId: string;
  success: boolean;
  result?: unknown;
  error?: string;
}

// ─── Canvas Context ───────────────────────────────────────────────────────────

export interface CanvasContext {
  elements: readonly OrderedExcalidrawElement[];
  activeFileId: string;
  activeTabId: string;
  roomId: string;
}

// ─── AI Config ───────────────────────────────────────────────────────────────

export type AIProvider = "openai" | "anthropic";

export interface AIConfig {
  provider: AIProvider;
  model: string;
  apiKey?: string; // Only stored server-side
  baseURL?: string; // For OpenAI-compatible endpoints
  maxTokens: number;
  temperature: number;
}

// ─── SSE Events ────────────────────────────────────────────────────────────────

export type SSEEventType = "text" | "tool_call" | "tool_result" | "usage" | "done" | "error";

export interface SSETextEvent {
  type: "text";
  content: string;
}

export interface SSEToolCallEvent {
  type: "tool_call";
  id: string;
  name: string;
  arguments: Record<string, unknown>;
}

export interface SSEToolResultEvent {
  type: "tool_result";
  callId: string;
  name: string;
  success: boolean;
  result?: unknown;
  error?: string;
}

export interface SSEDoneEvent {
  type: "done";
  summary: string;
  elementCount: number;
}

export interface SSEUsageEvent {
  type: "usage";
  usage: TokenUsage;
}

export interface SSEErrorEvent {
  type: "error";
  message: string;
}

export type SSEEvent =
  | SSETextEvent
  | SSEToolCallEvent
  | SSEToolResultEvent
  | SSEUsageEvent
  | SSEDoneEvent
  | SSEErrorEvent;

// ─── AI Tools (MCP) ──────────────────────────────────────────────────────────

export interface CreateRectangleParams {
  x: number;
  y: number;
  width: number;
  height: number;
  label?: string;
  backgroundColor?: string;
  strokeColor?: string;
}

export interface CreateTextParams {
  x: number;
  y: number;
  text: string;
  fontSize?: number;
  strokeColor?: string;
}

export interface CreateArrowParams {
  startX: number;
  startY: number;
  endX: number;
  endY: number;
  label?: string;
  strokeColor?: string;
}

export interface MoveElementsParams {
  elementIds: string[];
  deltaX: number;
  deltaY: number;
}

export interface DeleteElementsParams {
  elementIds: string[];
}

export interface EditTextParams {
  elementId: string;
  text?: string;
  fontSize?: number;
  strokeColor?: string;
}

export interface MermaidToExcalidrawParams {
  syntax: string;
  x?: number;
  y?: number;
}

export interface AutoLayoutParams {
  elementIds?: string[];
  layout: "vertical" | "horizontal" | "grid";
  spacing?: number;
  columns?: number;
  originX?: number;
  originY?: number;
}

export interface GetCanvasStateResult {
  elementCount: number;
  elementTypes: string[];
  boundingBox: { x: number; y: number; width: number; height: number } | null;
}
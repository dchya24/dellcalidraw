import type {
  SSEEvent,
  CanvasContext,
} from "../../types/ai";

function getBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_URL;
  if (envUrl) return envUrl;

  const protocol = window.location.protocol === "https:" ? "https:" : "http:";
  const host = window.location.hostname;
  return `${protocol}//${host}:8080`;
}

// ─── SSE line parser ─────────────────────────────────────────────────────────

function parseSSEEventLine(data: unknown): SSEEvent | null {
  if (typeof data !== "object" || data === null) return null;

  const obj = data as Record<string, unknown>;

  // Tool call event
  if (
    obj.type === "tool_call" ||
    typeof obj.name === "string" &&
    (obj.name.startsWith("create_") ||
      obj.name.startsWith("delete_") ||
      obj.name.startsWith("move_") ||
      obj.name.includes("mermaid"))
  ) {
    return {
      type: "tool_call",
      id: String(obj.id || obj.callId || crypto.randomUUID()),
      name: String(obj.name || obj.tool || ""),
      arguments: (obj.arguments || obj.params || obj) as Record<string, unknown>,
    };
  }

  // Tool result event
  if (obj.type === "tool_result") {
    return {
      type: "tool_result",
      callId: String(obj.callId || obj.id || ""),
      name: String(obj.name || ""),
      success: obj.success !== false,
      result: obj.result,
      error: obj.error as string | undefined,
    };
  }

  // Done event
  if (obj.type === "done" || obj.done === true) {
    return {
      type: "done",
      summary: String(obj.summary || obj.message || ""),
      elementCount: Number(obj.elementCount || 0),
    };
  }

  // Error event
  if (obj.type === "error") {
    return {
      type: "error",
      message: String(obj.message || "Unknown error"),
    };
  }

  // Text event
  const text = String(
    obj.content || obj.text || obj.message || obj.data || ""
  );
  if (text) {
    return { type: "text", content: text };
  }

  return null;
}

// ─── Send message to AI ──────────────────────────────────────────────────────

export interface ChatOptions {
  message: string;
  canvasContext: CanvasContext;
  model?: string;
  onEvent: (event: SSEEvent) => void;
  onError: (error: Error) => void;
  onComplete: () => void;
  signal?: AbortSignal;
}

export async function sendChatMessage(options: ChatOptions): Promise<void> {
  const { message, canvasContext, model, onEvent, onError, onComplete, signal } =
    options;
  const baseUrl = getBaseUrl();

  try {
    const response = await fetch(`${baseUrl}/api/ai/chat`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getAuthHeaders(),
      },
      body: JSON.stringify({
        message,
        canvasContext: {
          elements: canvasContext.elements,
          activeFileId: canvasContext.activeFileId,
          activeTabId: canvasContext.activeTabId,
          roomId: canvasContext.roomId,
        },
        model: model || "gpt-4o",
      }),
      signal,
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`AI request failed: ${response.status} ${errorText}`);
    }

    // SSE streaming
    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body");
    }

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      if (signal?.aborted) break;

      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          const eventData = line.substring(6);
          try {
            const jsonData = JSON.parse(eventData);
            const event = parseSSEEventLine(jsonData);
            if (event) {
              onEvent(event);
            }
          } catch {
            // Skip malformed JSON
          }
        }
      }
    }

    onComplete();
  } catch (err) {
    onError(err as Error);
  }
}

// ─── List available models ────────────────────────────────────────────────────

export async function listModels(): Promise<string[]> {
  const baseUrl = getBaseUrl();

  try {
    const response = await fetch(`${baseUrl}/api/ai/models`, {
      headers: getAuthHeaders(),
    });

    if (!response.ok) return [];

    const data = await response.json();
    return data.models || [];
  } catch {
    return [];
  }
}

// ─── Helper: get auth headers ─────────────────────────────────────────────────

function getAuthHeaders(): Record<string, string> {
  try {
    const raw = localStorage.getItem("auth-storage");
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    const token = parsed?.state?.accessToken;
    if (!token) return {};
    return { Authorization: `Bearer ${token}` };
  } catch {
    return {};
  }
}

// ─── Send message (non-streaming, for quick testing) ─────────────────────────

export async function sendSimpleChat(
  message: string,
  canvasContext: CanvasContext
): Promise<{ response: string; elements?: unknown[] }> {
  const baseUrl = getBaseUrl();

  const response = await fetch(`${baseUrl}/api/ai/chat`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
    },
    body: JSON.stringify({
      message,
      canvasContext,
      stream: false,
    }),
  });

  if (!response.ok) {
    throw new Error(`AI request failed: ${response.status}`);
  }

  return response.json();
}
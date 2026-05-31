import type {
  SSEEvent,
  CanvasContext,
} from "../../types/ai";

function getBaseUrl(): string {
  // Priority 1: Environment variable (for production)
  const envUrl = import.meta.env.VITE_API_URL;
  if (envUrl) {
    console.log("[AI Service] Using VITE_API_URL:", envUrl);
    return envUrl;
  }

  // For development: use direct backend URL (CORS is configured on backend)
  const protocol = window.location.protocol === "https:" ? "https:" : "http:";
  const host = window.location.hostname;
  // Use port 8080 for backend
  return `${protocol}//${host}:8080`;
}

// ─── SSE event parser ────────────────────────────────────────────────────────

function parseSSEEvent(data: unknown): SSEEvent | null {
  if (typeof data !== "object" || data === null) return null;

  const obj = data as Record<string, unknown>;

  // Tool call event
  if (obj.type === "tool_call") {
    return {
      type: "tool_call",
      id: String(obj.id || obj.callId || crypto.randomUUID()),
      name: String(obj.name || ""),
      arguments: (obj.result || obj.arguments || obj.params || {}) as Record<string, unknown>,
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
      summary: String(obj.summary || obj.content || ""),
      elementCount: Number(obj.elementCount || 0),
    };
  }

  // Usage event — backend emits totals once per request
  if (obj.type === "usage" && obj.usage && typeof obj.usage === "object") {
    const u = obj.usage as Record<string, unknown>;
    return {
      type: "usage",
      usage: {
        promptTokens: Number(u.promptTokens || 0),
        completionTokens: Number(u.completionTokens || 0),
        totalTokens: Number(u.totalTokens || 0),
      },
    };
  }

  // Error event
  if (obj.type === "error") {
    return {
      type: "error",
      message: String(obj.content || obj.message || "Unknown error"),
    };
  }

  // Text event
  const text = String(obj.content || obj.text || obj.message || "");
  if (text) {
    return { type: "text", content: text };
  }

  return null;
}

// ─── Send message to AI ──────────────────────────────────────────────────────

export interface ChatOptions {
  message: string;
  model?: string;
  canvasContext: CanvasContext;
  onEvent: (event: SSEEvent) => void;
  onError: (error: Error) => void;
  onComplete: () => void;
  signal?: AbortSignal;
}

export async function sendChatMessage(options: ChatOptions): Promise<void> {
  const { message, model, canvasContext, onEvent, onError, onComplete, signal } = options;
  const baseUrl = getBaseUrl();

  // Create timeout controller (120 seconds max for AI response)
  const timeoutMs = 120000;
  const timeoutController = new AbortController();
  const timeoutId = setTimeout(() => {
    console.log("[AI Service] Timeout reached (120s), aborting request");
    timeoutController.abort();
  }, timeoutMs);

  // Combine signals: external signal + timeout
  const combinedSignal = signal
    ? (signal as AbortSignal)
    : timeoutController.signal;

  try {
    const fullUrl = `${baseUrl}/api/ai/chat`;
    console.log("[AI Service] Full URL:", fullUrl);
    console.log("[AI Service] Window location:", window.location.href);
    console.log("[AI Service] Request starting at:", Date.now());

    const response = await fetch(fullUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getAuthHeaders(),
      },
      body: JSON.stringify({
        message,
        model: model || undefined,
        canvasContext: {
          elements: canvasContext.elements,
          activeFileId: canvasContext.activeFileId,
          activeTabId: canvasContext.activeTabId,
          roomId: canvasContext.roomId,
        },
      }),
      signal: combinedSignal,
      // Important for SSE: keep connection alive
      keepalive: true,
    });

    // Clear timeout once we get a response
    clearTimeout(timeoutId);
    console.log("[AI Service] Response status:", response.status);
    console.log("[AI Service] Response type:", response.type);

    if (!response.ok) {
      const errorText = await response.text();
      console.error("[AI Service] Error response:", response.status, errorText);
      throw new Error(`AI request failed: ${response.status} ${errorText}`);
    }

    // SSE streaming via ReadableStream
    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body for SSE stream");
    }

    console.log('reader', reader);

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      // Check if aborted
      if (combinedSignal.aborted) {
        console.log("[AI Service] Request aborted");
        break;
      }

      // Read from stream with timeout check
      const readWithTimeout = new Promise<{ done: boolean; value?: Uint8Array }>((resolve, reject) => {
        const timeout = setTimeout(() => {
          reject(new Error("Read timeout"));
        }, 60000); // 60 second read timeout

        reader.read().then(
          (result) => {
            console.log('result reader', result)
            clearTimeout(timeout);
            resolve(result);
          },
          (err) => {
            console.log('reader error', err)
            clearTimeout(timeout);
            reject(err);
          }
        ).catch((err) => {
          console.log('reader catch', err)
        });
      });

      let result;
      try {
        result = await readWithTimeout;
      } catch (err) {
        console.error("[AI Service] Read error:", err);
        if (combinedSignal.aborted) break;
        throw err;
      }

      console.log('result readWithTimeout', result)


      const { done, value } = result;
      if (done) {
        console.log("[AI Service] Stream complete");
        break;
      }

      // Decode and process
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed.startsWith("data: ")) continue;

        const eventData = trimmed.substring(6);
        if (!eventData || eventData === "[DONE]") continue;

        try {
          const jsonData = JSON.parse(eventData);
          const event = parseSSEEvent(jsonData);
          if (event) {
            onEvent(event);
          }
        } catch {
          // Skip malformed JSON
        }
      }
    }

    onComplete();
  } catch (err) {
    clearTimeout(timeoutId);
    console.error("[AI Service] Error:", err);

    // Check if timeout was the cause
    if (combinedSignal.aborted && !(signal?.aborted)) {
      onError(new Error("Request timed out. The AI is taking too long to respond."));
      return;
    }

    if (signal?.aborted) {
      onComplete();
      return;
    }

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

// ─── Get active model from server ────────────────────────────────────────────

export async function getActiveModel(): Promise<string> {
  const baseUrl = getBaseUrl();

  try {
    const response = await fetch(`${baseUrl}/api/ai/health`, {
      headers: getAuthHeaders(),
    });

    if (!response.ok) return "";

    const data = await response.json();
    return data.activeModel || "";
  } catch {
    return "";
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

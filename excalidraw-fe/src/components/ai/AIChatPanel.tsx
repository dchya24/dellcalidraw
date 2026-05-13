import { useRef, useEffect, useState, useCallback } from "react";
import {
  Send,
  X,
  Bot,
  User,
  Loader2,
  Sparkles,
  Trash2,
} from "lucide-react";
import { useAIChatStore } from "../../store/useAIChatStore";
import { useWhiteboardStore } from "../../store/useWhiteboardStore";
import { sendChatMessage } from "../../services/ai/aiService";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import type { ChatMessage } from "../../types/ai";

interface AIChatPanelProps {
  excalidrawAPI: ExcalidrawImperativeAPI | null;
}

export default function AIChatPanel({ excalidrawAPI }: AIChatPanelProps) {
  const {
    isPanelOpen,
    isStreaming,
    togglePanel,
    initConversation,
    addMessage,
    clearConversation,
    setStreaming,
    conversations,
  } = useAIChatStore();

  const { getActiveFile, getActiveTab, saveTabState } = useWhiteboardStore();

  const [inputValue, setInputValue] = useState("");
  const [currentTabId, setCurrentTabId] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  // Initialize conversation when tab changes
  useEffect(() => {
    const activeFile = getActiveFile();
    if (!activeFile) return;

    const tabId = activeFile.activeTabId;
    if (tabId !== currentTabId) {
      setCurrentTabId(tabId);
      initConversation(tabId);
    }
  }, [getActiveFile, currentTabId, initConversation]);

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [conversations, currentTabId]);

  // Focus input when panel opens
  useEffect(() => {
    if (isPanelOpen && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isPanelOpen]);

  // Cleanup abort controller on unmount
  useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  // Get messages for current tab
  const messages = currentTabId ? conversations[currentTabId] || [] : [];

  // Send message to AI
  const handleSend = useCallback(async () => {
    if (!inputValue.trim() || !currentTabId) return;
    if (isStreaming) return;

    const userMessage: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: inputValue.trim(),
      timestamp: Date.now(),
    };

    addMessage(currentTabId, userMessage);
    setInputValue("");
    setStreaming(true);

    // Create abort controller for this request
    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();

    const activeFile = getActiveFile();
    const activeTab = getActiveTab();

    if (!activeFile || !activeTab || !excalidrawAPI) {
      addMessage(currentTabId, {
        id: crypto.randomUUID(),
        role: "assistant",
        content: "Error: Canvas not ready. Please try again.",
        timestamp: Date.now(),
      });
      setStreaming(false);
      return;
    }

    // Save current canvas state before AI operations
    const elements = excalidrawAPI.getSceneElements();
    const appState = excalidrawAPI.getAppState();
    const files = excalidrawAPI.getFiles();
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { collaborators, ...safeAppState } = appState;
    saveTabState(activeTab.id, elements, safeAppState, files);

    // Create assistant message placeholder for streaming updates
    const assistantMsgId = crypto.randomUUID();
    addMessage(currentTabId, {
      id: assistantMsgId,
      role: "assistant",
      content: "",
      timestamp: Date.now(),
    });

    await sendChatMessage({
      message: userMessage.content,
      canvasContext: {
        elements,
        activeFileId: activeFile.id,
        activeTabId: activeTab.id,
        roomId: activeTab.roomId,
      },
      onEvent: (event) => {
        if (!excalidrawAPI) return;

        switch (event.type) {
          case "text":
            // Update assistant message with streaming text
            addMessage(currentTabId, {
              id: assistantMsgId,
              role: "assistant",
              content: event.content,
              timestamp: Date.now(),
            });
            break;

          case "tool_call": {
            const args = (event.arguments || {}) as Record<string, unknown>;
            const toolName = event.name;

            // Handle modify tools
            if (toolName === "move_elements") {
              applyMoveElements(excalidrawAPI, args);
              break;
            }
            if (toolName === "delete_elements") {
              applyDeleteElements(excalidrawAPI, args);
              break;
            }
            if (toolName === "update_element_style") {
              applyUpdateStyle(excalidrawAPI, args);
              break;
            }

            // Handle create tools — add to canvas immediately (streaming)
            const element = generateElementFromTool(toolName, args);
            if (element) {
              const currentElements = excalidrawAPI.getSceneElements();
              excalidrawAPI.updateScene({
                elements: [...currentElements, element as Parameters<typeof excalidrawAPI.updateScene>[0]["elements"][number]],
              });
            }
            break;
          }

          case "tool_result":
            break;

          case "done":
            if (excalidrawAPI) {
              excalidrawAPI.history.clear();
            }
            break;
        }
      },
      onError: (error) => {
        addMessage(currentTabId, {
          id: assistantMsgId,
          role: "assistant",
          content: `Error: ${error.message}`,
          timestamp: Date.now(),
        });
      },
      onComplete: () => {
        setStreaming(false);
      },
      signal: abortControllerRef.current.signal,
    });
  }, [
    inputValue,
    currentTabId,
    isStreaming,
    addMessage,
    setStreaming,
    getActiveFile,
    getActiveTab,
    saveTabState,
    excalidrawAPI,
  ]);

  // Handle keyboard shortcuts
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // Suggested prompts
  const suggestedPrompts = [
    "Buatkan flowchart login sederhana",
    "Tambahkan diagram proses checkout",
    "Buatkan ERD untuk aplikasi e-commerce",
  ];

  if (!isPanelOpen) {
    return (
      <button
        onClick={togglePanel}
        className="fixed bottom-16 right-4 z-50 p-3 rounded-full shadow-lg transition-all hover:scale-105"
        style={{
          background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
          color: "white",
        }}
        title="Open AI Assistant"
      >
        <Sparkles size={20} />
      </button>
    );
  }

  return (
    <div className="fixed bottom-16 right-4 w-96 h-[500px] max-h-[70vh] z-50 rounded-2xl shadow-2xl flex flex-col overflow-hidden"
      style={{
        background: "var(--bg-color, white)",
        border: "1px solid var(--border-color, #e5e7eb)",
      }}>
      {/* Header */}
      <div
        className="flex items-center justify-between px-4 py-3 border-b"
        style={{ borderColor: "var(--border-color, #e5e7eb)" }}
      >
        <div className="flex items-center gap-2">
          <div
            className="p-1.5 rounded-lg"
            style={{
              background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
            }}
          >
            <Sparkles size={16} className="text-white" />
          </div>
          <span className="font-semibold text-sm">AI Assistant</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => currentTabId && clearConversation(currentTabId)}
            className="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            title="Clear conversation"
          >
            <Trash2 size={14} />
          </button>
          <button
            onClick={togglePanel}
            className="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          >
            <X size={18} />
          </button>
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 && (
          <div className="text-center py-8">
            <Bot size={32} className="mx-auto text-gray-400 mb-2" />
            <p className="text-sm text-gray-500">
              Halo! Saya bisa membantu membuat diagram.
            </p>
          </div>
        )}

        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
          >
            <div
              className={`max-w-[85%] rounded-2xl px-4 py-2.5 ${
                msg.role === "user"
                  ? "bg-blue-500 text-white rounded-br-md"
                  : "bg-gray-100 dark:bg-gray-800 rounded-bl-md"
              }`}
            >
              <div className="flex items-start gap-2">
                {msg.role === "assistant" && (
                  <Bot size={14} className="mt-1 flex-shrink-0 opacity-60" />
                )}
                {msg.role === "user" && (
                  <User size={14} className="mt-1 flex-shrink-0 opacity-80" />
                )}
                <div className="text-sm whitespace-pre-wrap">
                  {msg.content}
                  {msg.toolCalls && msg.toolCalls.length > 0 && (
                    <div className="mt-2 text-xs opacity-70">
                      {msg.toolCalls.map((tc, i) => (
                        <span key={i} className="inline-block px-2 py-1 rounded bg-black/10 mr-1 mb-1">
                          {tc.name}()
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>
              {msg.toolResults && msg.toolResults.length > 0 && (
                <div className="mt-2 text-xs">
                  {msg.toolResults?.map((tr, i) => (
                    <div
                      key={i}
                      className={`mt-1 px-2 py-1 rounded text-xs ${
                        tr.success ? "bg-green-500/20" : "bg-red-500/20"
                      }`}
                    >
                      {tr.success ? "✓" : "✗"} {String(tr.callId)}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        ))}

        {/* Suggested prompts (when empty) */}
        {messages.length <= 1 && !isStreaming && (
          <div className="space-y-2">
            <p className="text-xs text-gray-400 text-center">Suggested:</p>
            <div className="flex flex-wrap gap-2 justify-center">
              {suggestedPrompts.map((prompt, i) => (
                <button
                  key={i}
                  onClick={() => {
                    setInputValue(prompt);
                    inputRef.current?.focus();
                  }}
                  className="text-xs px-3 py-1.5 rounded-full bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                >
                  {prompt}
                </button>
              ))}
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div
        className="p-3 border-t"
        style={{ borderColor: "var(--border-color, #e5e7eb)" }}
      >
        <div className="flex items-end gap-2">
          <textarea
            ref={inputRef}
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask AI to create or modify diagram..."
            className="flex-1 resize-none rounded-xl px-4 py-2.5 text-sm outline-none focus:ring-2 focus:ring-blue-500/50"
            style={{
              background: "var(--input-bg, #f9fafb)",
              minHeight: "44px",
              maxHeight: "120px",
            }}
            rows={1}
            disabled={isStreaming}
          />
          <button
            onClick={handleSend}
            disabled={!inputValue.trim() || isStreaming}
            className="p-2.5 rounded-xl bg-blue-500 text-white hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isStreaming ? (
              <Loader2 size={18} className="animate-spin" />
            ) : (
              <Send size={18} />
            )}
          </button>
        </div>

        {/* Stop button when streaming */}
        {isStreaming && (
          <button
            onClick={() => abortControllerRef.current?.abort()}
            className="mt-2 text-xs text-gray-500 hover:text-red-500 flex items-center gap-1"
          >
            <X size={12} />
            Stop generating
          </button>
        )}
      </div>
    </div>
  );
}

// ─── Element Generator from Tool Calls ───────────────────────────────────────

function generateElementFromTool(
  toolName: string,
  args: Record<string, unknown>
): unknown {
  const base = {
    id: crypto.randomUUID(),
    seed: Math.floor(Math.random() * 1000000),
    version: 1,
    versionNonce: Math.floor(Math.random() * 1000000),
    updated: Date.now(),
    isDeleted: false,
    groupIds: [],
    frameId: null,
    boundElements: null,
    link: null,
    locked: false,
    roundness: null,
  };

  switch (toolName) {
    case "create_rectangle": {
      return {
        ...base,
        type: "rectangle" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: Number(args.width || 120),
        height: Number(args.height || 60),
        angle: 0,
        strokeColor: String(args.strokeColor || "#000000"),
        backgroundColor: String(args.backgroundColor || "#ffffff"),
        fillStyle: "solid" as const,
        strokeWidth: 1,
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: 100,
      } as unknown;
    }

    case "create_text": {
      return {
        ...base,
        type: "text" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: 200,
        height: 30,
        angle: 0,
        strokeColor: String(args.strokeColor || "#000000"),
        backgroundColor: "transparent",
        fillStyle: "solid" as const,
        strokeWidth: 0,
        strokeStyle: "solid" as const,
        roughness: 0,
        opacity: 100,
        text: String(args.text || ""),
        fontSize: Number(args.fontSize || 16),
      } as unknown;
    }

    case "create_arrow": {
      return {
        ...base,
        type: "arrow" as const,
        x: Number(args.startX || 0),
        y: Number(args.startY || 0),
        width: Math.abs(Number(args.endX || 100) - Number(args.startX || 0)),
        height: Math.abs(Number(args.endY || 50) - Number(args.startY || 0)),
        angle: 0,
        strokeColor: String(args.strokeColor || "#000000"),
        backgroundColor: "transparent",
        fillStyle: "solid" as const,
        strokeWidth: 1,
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: 100,
        points: [
          [0, 0] as [number, number],
          [
            Number(args.endX || 100) - Number(args.startX || 0),
            Number(args.endY || 50) - Number(args.startY || 0),
          ] as [number, number],
        ],
        lastCommittedPoint: null,
        startBinding: null,
        endBinding: null,
        startArrowhead: null,
        endArrowhead: "arrow" as const,
      } as unknown;
    }

    case "create_ellipse": {
      return {
        ...base,
        type: "ellipse" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: Number(args.width || 100),
        height: Number(args.height || 60),
        angle: 0,
        strokeColor: String(args.strokeColor || "#000000"),
        backgroundColor: String(args.backgroundColor || "#ffffff"),
        fillStyle: "solid" as const,
        strokeWidth: 1,
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: 100,
      } as unknown;
    }

    case "create_diamond": {
      return {
        ...base,
        type: "diamond" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: Number(args.width || 100),
        height: Number(args.height || 60),
        angle: 0,
        strokeColor: String(args.strokeColor || "#000000"),
        backgroundColor: String(args.backgroundColor || "#ffffff"),
        fillStyle: "solid" as const,
        strokeWidth: 1,
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: 100,
      } as unknown;
    }

    case "create_line": {
      const rawPoints = args.points as [number, number][] || [[0, 0], [100, 0]];
      return {
        ...base,
        type: "line" as const,
        x: rawPoints[0]?.[0] || 0,
        y: rawPoints[0]?.[1] || 0,
        width: 100,
        height: 0,
        angle: 0,
        strokeColor: String(args.strokeColor || "#000000"),
        backgroundColor: "transparent",
        fillStyle: "solid" as const,
        strokeWidth: 1,
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: 100,
        points: rawPoints.map((p, i) =>
          i === 0 ? [0, 0] : [p[0] - rawPoints[0][0], p[1] - rawPoints[0][1]]
        ),
        lastCommittedPoint: null,
        startBinding: null,
        endBinding: null,
        startArrowhead: null,
        endArrowhead: null,
      } as unknown;
    }

    default:
      return null;
  }
}

// ─── Modify Tool Helpers ─────────────────────────────────────────────────────

function applyMoveElements(
  api: ExcalidrawImperativeAPI,
  args: Record<string, unknown>
): void {
  const ids = (args.elementIds as string[]) || [];
  const dx = Number(args.deltaX || 0);
  const dy = Number(args.deltaY || 0);
  if (ids.length === 0) return;

  const elements = api.getSceneElements();
  const updated = elements.map((el) => {
    if (ids.includes(el.id)) {
      return { ...el, x: el.x + dx, y: el.y + dy };
    }
    return el;
  });
  api.updateScene({ elements: updated });
}

function applyDeleteElements(
  api: ExcalidrawImperativeAPI,
  args: Record<string, unknown>
): void {
  const ids = (args.elementIds as string[]) || [];
  if (ids.length === 0) return;

  const elements = api.getSceneElements();
  const updated = elements.map((el) => {
    if (ids.includes(el.id)) {
      return { ...el, isDeleted: true };
    }
    return el;
  });
  api.updateScene({ elements: updated });
}

function applyUpdateStyle(
  api: ExcalidrawImperativeAPI,
  args: Record<string, unknown>
): void {
  const ids = (args.elementIds as string[]) || [];
  if (ids.length === 0) return;

  const elements = api.getSceneElements();
  const updated = elements.map((el) => {
    if (!ids.includes(el.id)) return el;

    const patched = { ...el } as Record<string, unknown>;
    if (args.backgroundColor !== undefined) patched.backgroundColor = String(args.backgroundColor);
    if (args.strokeColor !== undefined) patched.strokeColor = String(args.strokeColor);
    if (args.strokeWidth !== undefined) patched.strokeWidth = Number(args.strokeWidth);
    if (args.opacity !== undefined) patched.opacity = Number(args.opacity);
    return patched;
  });
  api.updateScene({ elements: updated as typeof elements });
}
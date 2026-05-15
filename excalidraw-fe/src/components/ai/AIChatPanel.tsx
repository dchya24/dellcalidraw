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
    const activeFileFromEffect = getActiveFile();
    if (!activeFileFromEffect) {
      return;
    }

    const tabId = activeFileFromEffect.activeTabId;
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
          case "text": {
            // Update the last message with accumulated text
            // Use updateLastMessage instead of addMessage to avoid duplicate keys
            const { conversations } = useAIChatStore.getState();
            const msgs = conversations[currentTabId] || [];
            const last = msgs[msgs.length - 1];
            
            if (last && last.role === "assistant" && last.id === assistantMsgId) {
              // Update existing message with accumulated text
              const accumulated = (last.content || "") + event.content;
              addMessage(currentTabId, {
                id: assistantMsgId,
                role: "assistant",
                content: accumulated,
                timestamp: Date.now(),
              });
            } else {
              // First text event - create message with this content
              addMessage(currentTabId, {
                id: assistantMsgId,
                role: "assistant",
                content: event.content,
                timestamp: Date.now(),
              });
            }
            break;
          }

          case "tool_call": {
            const args = (event.arguments || {}) as Record<string, unknown>;
            const toolName = event.name;

            // Handle camera_update - just log it for now (viewport is auto-managed)
            if (toolName === "camera_update") {
              console.log("[AIChatPanel] Camera update requested:", args);
              break;
            }

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

            // Handle create tools — add to canvas immediately
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
    <div className="fixed bottom-16 right-4 w-96 h-125 max-h-[70vh] z-50 rounded-2xl shadow-2xl flex flex-col overflow-hidden"
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
            className="p-1.5 rounded-lg hover:bg-gray-100 transition-colors"
            title="Clear conversation"
          >
            <Trash2 size={14} />
          </button>
          <button
            onClick={togglePanel}
            className="p-1.5 rounded-lg hover:bg-gray-100 transition-colors"
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
                  : "bg-gray-100 rounded-bl-md"
              }`}
            >
              <div className="flex items-start gap-2">
                {msg.role === "assistant" && (
                  <Bot size={14} className="mt-1 shrink-0 opacity-60" />
                )}
                {msg.role === "user" && (
                  <User size={14} className="mt-1 shrink-0 opacity-80" />
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
                  className="text-xs px-3 py-1.5 rounded-full bg-gray-100 hover:bg-gray-200 transition-colors cursor-pointer"
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

interface LabelObject {
  text: string;
  fontSize?: number;
}

interface BindingObject {
  elementId: string;
  fixedPoint: [number, number];
}

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
  };

  switch (toolName) {
    case "camera_update": {
      // Camera update - handled separately, return null for elements
      return null;
    }

    case "create_rectangle": {
      const label = args.label as LabelObject | undefined;
      const roundness = args.roundness as { type: number } | undefined;
      const fontSize = label?.fontSize || 18;
      const text = label?.text || "";
      
      return {
        ...base,
        type: "rectangle" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: Math.max(Number(args.width || 120), 120),
        height: Math.max(Number(args.height || 60), 60),
        angle: 0,
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        backgroundColor: String(args.backgroundColor || "transparent"),
        fillStyle: (args.fillStyle as "solid" | "hatching" | "cross-hatch") || "solid",
        strokeWidth: Number(args.strokeWidth || 2),
        strokeStyle: "solid" as const,
        roughness: Number(args.roughness ?? 1),
        opacity: Number(args.opacity ?? 100),
        roundness: roundness ? { type: roundness.type } : null,
        label: text ? { text, fontSize } : undefined,
      };
    }

    case "create_text": {
      const fontSize = Number(args.fontSize || 20);
      const text = String(args.text || "");
      const width = Math.max(text.length * fontSize * 0.6, 100);
      
      return {
        ...base,
        type: "text" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width,
        height: fontSize * 1.4,
        angle: 0,
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        backgroundColor: "transparent",
        fillStyle: "solid" as const,
        strokeWidth: 0,
        strokeStyle: "solid" as const,
        roughness: 0,
        opacity: 100,
        fontSize,
        text,
      };
    }

    case "create_arrow": {
      const label = args.label as LabelObject | undefined;
      const startBinding = args.startBinding as BindingObject | undefined;
      const endBinding = args.endBinding as BindingObject | undefined;
      
      const startX = Number(args.startX || 0);
      const startY = Number(args.startY || 0);
      const endX = Number(args.endX || 100);
      const endY = Number(args.endY || 50);
      
      return {
        ...base,
        type: "arrow" as const,
        x: Math.min(startX, endX),
        y: Math.min(startY, endY),
        width: Math.abs(endX - startX),
        height: Math.abs(endY - startY),
        angle: 0,
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        backgroundColor: "transparent",
        fillStyle: "solid" as const,
        strokeWidth: Number(args.strokeWidth || 2),
        strokeStyle: (args.strokeStyle as "solid" | "dashed" | "dotted") || "solid",
        roughness: 1,
        opacity: 100,
        points: [[0, 0], [endX - startX, endY - startY]] as [number, number][],
        lastCommittedPoint: null,
        startBinding: startBinding || null,
        endBinding: endBinding || null,
        startArrowhead: (args.startArrowhead as "arrow" | "bar" | "dot" | "triangle" | null) || null,
        endArrowhead: (args.endArrowhead as "arrow" | "bar" | "dot" | "triangle" | null) || "arrow",
        label: label ? { text: label.text, fontSize: label.fontSize || 14 } : undefined,
      };
    }

    case "create_ellipse": {
      const label = args.label as LabelObject | undefined;
      
      return {
        ...base,
        type: "ellipse" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: Math.max(Number(args.width || 120), 120),
        height: Math.max(Number(args.height || 60), 60),
        angle: 0,
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        backgroundColor: String(args.backgroundColor || "transparent"),
        fillStyle: (args.fillStyle as "solid" | "hatching" | "cross-hatch") || "solid",
        strokeWidth: Number(args.strokeWidth || 2),
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: Number(args.opacity ?? 100),
        label: label ? { text: label.text, fontSize: label.fontSize || 18 } : undefined,
      };
    }

    case "create_diamond": {
      const label = args.label as LabelObject | undefined;
      
      return {
        ...base,
        type: "diamond" as const,
        x: Number(args.x || 100),
        y: Number(args.y || 100),
        width: Math.max(Number(args.width || 120), 120),
        height: Math.max(Number(args.height || 80), 80),
        angle: 0,
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        backgroundColor: String(args.backgroundColor || "transparent"),
        fillStyle: (args.fillStyle as "solid" | "hatching" | "cross-hatch") || "solid",
        strokeWidth: Number(args.strokeWidth || 2),
        strokeStyle: "solid" as const,
        roughness: 1,
        opacity: Number(args.opacity ?? 100),
        label: label ? { text: label.text, fontSize: label.fontSize || 16 } : undefined,
      };
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
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        backgroundColor: "transparent",
        fillStyle: "solid" as const,
        strokeWidth: Number(args.strokeWidth || 2),
        strokeStyle: (args.strokeStyle as "solid" | "dashed" | "dotted") || "solid",
        roughness: 1,
        opacity: 100,
        points: rawPoints.map((p, i) =>
          i === 0 ? [0, 0] : [p[0] - rawPoints[0][0], p[1] - rawPoints[0][1]]
        ),
        lastCommittedPoint: null,
        startBinding: null,
        endBinding: null,
        startArrowhead: (args.startArrowhead as "arrow" | "bar" | "dot" | "triangle" | null) || null,
        endArrowhead: (args.endArrowhead as "arrow" | "bar" | "dot" | "triangle" | null) || null,
      };
    }

    case "create_zone": {
      const label = args.label as LabelObject | undefined;
      
      return {
        ...base,
        type: "rectangle" as const,
        x: Number(args.x || 0),
        y: Number(args.y || 0),
        width: Number(args.width || 800),
        height: Number(args.height || 600),
        angle: 0,
        strokeColor: String(args.strokeColor || "#b0b0b0"),
        backgroundColor: String(args.backgroundColor || "#dbe4ff"),
        fillStyle: "solid" as const,
        strokeWidth: Number(args.strokeWidth || 1),
        strokeStyle: "solid" as const,
        roughness: 0,
        opacity: Number(args.opacity ?? 35),
        roundness: { type: 3 },
        label: label ? { text: label.text, fontSize: label.fontSize || 16 } : undefined,
      };
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

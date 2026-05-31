import { useRef, useEffect, useState, useCallback } from "react";
import {
  Send,
  X,
  Bot,
  User,
  Loader2,
  Sparkles,
  Trash2,
  ChevronDown,
  Undo2,
} from "lucide-react";
import { useAIChatStore } from "../../store/useAIChatStore";
import { useWhiteboardStore } from "../../store/useWhiteboardStore";
import { sendChatMessage, listModels, getActiveModel } from "../../services/ai/aiService";
import { convertToExcalidrawElements } from "@excalidraw/excalidraw";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import type { ChatMessage, ToolCall, TokenUsage } from "../../types/ai";

// Type alias - convertToExcalidrawElements accepts this skeleton format
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type ExcalidrawElementSkeleton = Parameters<typeof convertToExcalidrawElements>[0] extends (infer T)[] | null ? T : never;

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
    updateLastMessage,
    clearConversation,
    setStreaming,
    conversations,
  } = useAIChatStore();

  const { getActiveFile, getActiveTab, saveTabState } = useWhiteboardStore();

  const [inputValue, setInputValue] = useState("");
  const [currentTabId, setCurrentTabId] = useState<string | null>(null);
  const [modelList, setModelList] = useState<string[]>([]);
  const [activeModel, setActiveModel] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const trackedToolCalls = useRef<ToolCall[]>([]);
  const trackedElementIds = useRef<string[]>([]);
  const trackedUsage = useRef<TokenUsage | null>(null);
  const [hasError, setHasError] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

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

  // Fetch available models on mount
  useEffect(() => {
    async function fetchModels() {
      const [models, model] = await Promise.all([
        listModels(),
        getActiveModel(),
      ]);
      if (models.length > 0) setModelList(models);
      if (model) setActiveModel(model);
    }
    fetchModels();
  }, []);

  // Close model picker when clicking outside
  useEffect(() => {
    if (!showModelPicker) return;
    const handleClick = () => setShowModelPicker(false);
    document.addEventListener("click", handleClick);
    return () => document.removeEventListener("click", handleClick);
  }, [showModelPicker]);

  // Cleanup abort controller on unmount
  useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  // Get messages for current tab (deduplicate by id, keeping latest version)
  const rawMessages = currentTabId ? conversations[currentTabId] || [] : [];
  const messages = (() => {
    const seen = new Map<string, ChatMessage>();
    rawMessages.forEach(msg => seen.set(msg.id, msg));
    return Array.from(seen.values());
  })();

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
      model: activeModel || undefined,
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
            const { conversations: textConvs } = useAIChatStore.getState();
            const textMsgs = textConvs[currentTabId] || [];
            const textLast = textMsgs[textMsgs.length - 1];
            if (textLast && textLast.id === assistantMsgId) {
              updateLastMessage(currentTabId, assistantMsgId, {
                content: (textLast.content || "") + event.content,
              });
            }
            break;
          }

          case "tool_call": {
            const args = (event.arguments || {}) as Record<string, unknown>;
            const toolName = event.name;

            // Track tool call for progress display and summary
            trackedToolCalls.current.push({
              id: event.id || crypto.randomUUID(),
              name: toolName,
              arguments: event.arguments as Record<string, unknown>,
            });

            // Show progress indicator if no text content yet
            const { conversations: tcConvs } = useAIChatStore.getState();
            const tcMsgs = tcConvs[currentTabId] || [];
            const tcLast = tcMsgs[tcMsgs.length - 1];
            if (tcLast && tcLast.id === assistantMsgId && !tcLast.content) {
              const nonCamera = trackedToolCalls.current.filter(tc => tc.name !== "camera_update");
              if (nonCamera.length > 0) {
                updateLastMessage(currentTabId, assistantMsgId, {
                  content: `Membuat diagram... (${nonCamera.length} elemen diproses)`,
                });
              }
            }

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
            if (toolName === "edit_text") {
              applyEditText(excalidrawAPI, args);
              break;
            }
            if (toolName === "auto_layout") {
              const newIds = applyAutoLayout(excalidrawAPI, args);
              for (const id of newIds) {
                trackedElementIds.current.push(id);
              }
              break;
            }
            if (toolName === "convert_mermaid") {
              // Async — don't await; let stream continue. New IDs are tracked
              // when the parser resolves.
              applyConvertMermaid(excalidrawAPI, args)
                .then((newIds) => {
                  for (const id of newIds) {
                    trackedElementIds.current.push(id);
                  }
                })
                .catch((err) => {
                  console.error("[AIChatPanel] convert_mermaid failed:", err);
                });
              break;
            }

            // Handle create tools — add to canvas immediately
            const { skeleton, bindings } = generateSkeletonFromTool(toolName, args, excalidrawAPI);
            console.log(`[AIChatPanel] Tool: ${toolName}`, { args, skeleton, bindings });
            if (skeleton) {
              try {
                const currentElements = excalidrawAPI.getSceneElements();
                const converted = convertToExcalidrawElements(
                  [skeleton],
                  { regenerateIds: false }
                );
                console.log(`[AIChatPanel] Converted ${converted.length} elements:`, converted.map(e => ({ id: e.id, type: e.type, x: e.x, y: e.y, width: e.width, height: e.height, points: (e as any).points })));
                if (converted.length > 0) {
                  // Track created element IDs for undo support
                  for (const ce of converted) {
                    trackedElementIds.current.push(ce.id);
                  }

                  let allElements = [...currentElements, ...converted];

                  // If there are bindings to existing elements, wire them up manually
                  if (bindings) {
                    allElements = applyArrowBindings(
                      allElements,
                      converted[0].id,
                      bindings
                    );
                  }

                  excalidrawAPI.updateScene({
                    elements: allElements,
                  });
                }
              } catch (err) {
                console.error(`[AIChatPanel] Error converting ${toolName}:`, err);
              }
            }
            break;
          }

          case "tool_result":
            break;

          case "usage": {
            trackedUsage.current = event.usage;
            break;
          }

          case "done": {
            // Finalize: add tool calls to message + generate summary if no text
            const { conversations: doneConvs } = useAIChatStore.getState();
            const doneMsgs = doneConvs[currentTabId] || [];
            const doneLast = doneMsgs[doneMsgs.length - 1];

            if (doneLast && doneLast.id === assistantMsgId) {
              const updates: Partial<ChatMessage> = {
                toolCalls: [...trackedToolCalls.current],
                createdElementIds: [...trackedElementIds.current],
              };
              if (trackedUsage.current) {
                updates.usage = trackedUsage.current;
              }
              const isProgressMsg = doneLast.content?.startsWith("Membuat diagram");
              if (!doneLast.content || isProgressMsg) {
                updates.content = generateToolSummaryText(trackedToolCalls.current);
              }
              updateLastMessage(currentTabId, assistantMsgId, updates);
            }

            trackedToolCalls.current = [];
            trackedElementIds.current = [];
            trackedUsage.current = null;
            if (excalidrawAPI) {
              excalidrawAPI.history.clear();
            }
            break;
          }
          case 'error': {
            setHasError(true);
            setErrorMessage(event.message);
            updateLastMessage(currentTabId, assistantMsgId, { content: event.message });
            break;
          }
        }
      },
      onError: (error) => {
        trackedToolCalls.current = [];
        trackedElementIds.current = [];
        trackedUsage.current = null;
        console.log('error sendChatMessage', error)
        setHasError(true);
        setErrorMessage(error.message);
        updateLastMessage(currentTabId, assistantMsgId, { content: error.message });
      },
      onComplete: () => {
        trackedToolCalls.current = [];
        trackedElementIds.current = [];
        trackedUsage.current = null;
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
    updateLastMessage,
    excalidrawAPI,
    activeModel
  ]);

  // Handle keyboard shortcuts
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // Undo AI-generated elements for a specific message
  const handleUndoAI = useCallback((elementIds: string[]) => {
    if (!excalidrawAPI || elementIds.length === 0) return;

    const elements = excalidrawAPI.getSceneElements();
    const idSet = new Set(elementIds);

    // Find bound text elements of the deleted shapes too
    const boundTextIds = new Set<string>();
    for (const el of elements) {
      if (idSet.has(el.id)) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const bound = (el as any).boundElements;
        if (Array.isArray(bound)) {
          for (const b of bound) {
            if (b.type === "text") boundTextIds.add(b.id);
          }
        }
      }
    }

    const allIdsToDelete = new Set([...idSet, ...boundTextIds]);
    const updated = elements.map((el) => {
      if (allIdsToDelete.has(el.id)) {
        return { ...el, isDeleted: true };
      }
      return el;
    });
    excalidrawAPI.updateScene({ elements: updated });
    excalidrawAPI.history.clear();
  }, [excalidrawAPI]);

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
    <div className="fixed bottom-16 right-4 w-3/12 h-125 max-h-[70vh] z-50 rounded-2xl shadow-2xl flex flex-col overflow-hidden"
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
          {(() => {
            const total = sumSessionUsage(messages);
            if (total.totalTokens <= 0) return null;
            return (
              <span
                className="hidden sm:inline-flex items-center px-1.5 py-0.5 rounded-full bg-gray-100 text-[10px] text-gray-500 font-mono"
                title={`Session tokens: ${total.promptTokens} prompt + ${total.completionTokens} completion = ${total.totalTokens} total`}
              >
                {formatNumber(total.totalTokens)} tok
              </span>
            );
          })()}
        </div>
        <div className="flex items-center gap-1">
          {/* Model selector */}
          {modelList.length > 0 && (
            <div className="relative">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setShowModelPicker(!showModelPicker);
                }}
                className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs hover:bg-gray-100 transition-colors max-w-[160px]"
                title="Select model"
              >
                <span className="truncate text-gray-500">{activeModel || "model"}</span>
                <ChevronDown size={12} className="shrink-0 text-gray-400" />
              </button>
              {showModelPicker && (
                <div className="absolute right-0 top-full mt-1 w-52 bg-white rounded-lg shadow-lg border border-gray-200 py-1 z-50 max-h-60 overflow-y-auto">
                  {modelList.map((model) => (
                    <button
                      key={model}
                      onClick={(e) => {
                        e.stopPropagation();
                        setActiveModel(model);
                        setShowModelPicker(false);
                      }}
                      className={`w-full text-left px-3 py-1.5 text-xs hover:bg-blue-50 transition-colors ${
                        model === activeModel ? "bg-blue-50 text-blue-700 font-medium" : "text-gray-700"
                      }`}
                    >
                      {model}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
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

        {messages.map((msg, msgIndex) => {
          const isLastMsg = msgIndex === messages.length - 1;
          const isThinking = isStreaming && isLastMsg && msg.role === "assistant" && !msg.content;
          const showToolBadges = msg.role === "assistant" && msg.toolCalls && msg.toolCalls.length > 0;
          const toolSummary = showToolBadges ? getToolCallSummary(msg.toolCalls!) : [];

          return (
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
                  <div className="text-sm">
                    {isThinking && (
                      <div className="flex items-center gap-1.5 py-1">
                        <span className="w-1.5 h-1.5 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
                        <span className="w-1.5 h-1.5 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
                        <span className="w-1.5 h-1.5 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
                      </div>
                    )}
                    {msg.content && (
                      <div className="whitespace-pre-wrap">{msg.content}</div>
                    )}
                    {showToolBadges && (
                      <div className="mt-2 flex flex-wrap gap-1">
                        {toolSummary.map((item, i) => (
                          <span
                            key={i}
                            className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 text-xs"
                          >
                            {item.icon} {item.label}
                          </span>
                        ))}
                        {msg.createdElementIds && msg.createdElementIds.length > 0 && !isStreaming && (
                          <button
                            onClick={() => handleUndoAI(msg.createdElementIds!)}
                            className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-red-50 text-red-600 text-xs hover:bg-red-100 transition-colors cursor-pointer"
                            title={`Undo ${msg.createdElementIds.length} elements`}
                          >
                            <Undo2 size={10} />
                            Undo
                          </button>
                        )}
                      </div>
                    )}
                    {msg.role === "assistant" && msg.usage && msg.usage.totalTokens > 0 && (
                      <div className="mt-1.5 text-[10px] text-gray-400 font-mono">
                        {formatTokenUsage(msg.usage)}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>
          );
        })}

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
        {/* with button close */}
        {hasError && (
          <div className="text-red-500 mb-2 flex items-center gap-2">
            <span>{errorMessage}</span>
            <button onClick={() => {
              setHasError(false);
              setErrorMessage(null);
            }} className="text-xs px-3 py-1.5 rounded-full bg-gray-100 hover:bg-gray-200 transition-colors cursor-pointer">Close</button>
          </div>
        )}
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

// ─── AI Chat Helpers ───────────────────────────────────────────────────────

const TOOL_META: Record<string, { icon: string; label: string }> = {
  create_rectangle: { icon: "⬜", label: "Rectangle" },
  create_ellipse: { icon: "⭕", label: "Ellipse" },
  create_diamond: { icon: "🔷", label: "Diamond" },
  create_arrow: { icon: "➡️", label: "Arrow" },
  create_text: { icon: "📝", label: "Text" },
  create_line: { icon: "📏", label: "Line" },
  create_zone: { icon: "📦", label: "Zone" },
  move_elements: { icon: "↔️", label: "Moved" },
  delete_elements: { icon: "🗑️", label: "Deleted" },
  update_element_style: { icon: "🎨", label: "Styled" },
  edit_text: { icon: "✏️", label: "Edited" },
  camera_update: { icon: "📷", label: "Camera" },
  convert_mermaid: { icon: "🧬", label: "Mermaid" },
  auto_layout: { icon: "📏", label: "Auto-layout" },
};

function getToolCallSummary(toolCalls: ToolCall[]): { icon: string; label: string }[] {
  const counts: Record<string, number> = {};
  toolCalls.forEach(tc => {
    counts[tc.name] = (counts[tc.name] || 0) + 1;
  });

  return Object.entries(counts)
    .filter(([name]) => name !== "camera_update")
    .map(([name, count]) => {
      const meta = TOOL_META[name] || { icon: "🔧", label: name.replace(/_/g, " ") };
      return {
        icon: meta.icon,
        label: count > 1 ? `${count}× ${meta.label}` : meta.label,
      };
    });
}

function generateToolSummaryText(toolCalls: ToolCall[]): string {
  const summary = getToolCallSummary(toolCalls);
  if (summary.length === 0) {
    // Only camera_update was called, no actual elements created
    return "AI tidak membuat elemen diagram. Silakan coba lagi dengan instruksi yang lebih spesifik.";
  }

  const total = toolCalls.filter(tc => tc.name !== "camera_update").length;
  const parts = summary.map(s => s.label);
  return `Diagram berhasil dibuat dengan ${total} elemen: ${parts.join(", ")}`;
}

// ─── Token Usage Helpers ───────────────────────────────────────────────────────────────
function formatTokenUsage(usage: TokenUsage): string {
  const total = usage.totalTokens || usage.promptTokens + usage.completionTokens;
  if (total <= 0) return "";
  return `${formatNumber(usage.promptTokens)} in · ${formatNumber(usage.completionTokens)} out · ${formatNumber(total)} total`;
}

function sumSessionUsage(messages: ChatMessage[]): TokenUsage {
  return messages.reduce<TokenUsage>(
    (acc, m) => {
      if (m.usage) {
        acc.promptTokens += m.usage.promptTokens;
        acc.completionTokens += m.usage.completionTokens;
        acc.totalTokens += m.usage.totalTokens;
      }
      return acc;
    },
    { promptTokens: 0, completionTokens: 0, totalTokens: 0 },
  );
}

function formatNumber(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

// ─── Skeleton Generator from Tool Calls ──────────────────────────────────────
// Uses ExcalidrawElementSkeleton format which is then processed by
// convertToExcalidrawElements to produce proper elements with correct
// bindings, containerId, boundElements, etc.

interface LabelObject {
  text: string;
  fontSize?: number;
}

interface BindingObject {
  elementId: string;
  fixedPoint?: [number, number];
}

interface ArrowBindings {
  startElementId?: string;
  endElementId?: string;
}

interface SkeletonResult {
  skeleton: ExcalidrawElementSkeleton | null;
  bindings?: ArrowBindings;
}

function generateSkeletonFromTool(
  toolName: string,
  args: Record<string, unknown>,
  _excalidrawAPI: ExcalidrawImperativeAPI
): SkeletonResult {
  const id = crypto.randomUUID();

  switch (toolName) {
    case "camera_update": {
      return { skeleton: null };
    }

    case "create_rectangle": {
      const label = args.label as LabelObject | undefined;
      const roundness = args.roundness as { type: number } | undefined;

      return {
        skeleton: {
          type: "rectangle",
          id,
          x: Number(args.x || 100),
          y: Number(args.y || 100),
          width: Math.max(Number(args.width || 120), 120),
          height: Math.max(Number(args.height || 60), 60),
          strokeColor: String(args.strokeColor || "#1e1e1e"),
          backgroundColor: String(args.backgroundColor || "transparent"),
          fillStyle: (args.fillStyle as "solid" | "hachure" | "cross-hatch") || "solid",
          strokeWidth: Number(args.strokeWidth || 2),
          roughness: Number(args.roughness ?? 1),
          opacity: Number(args.opacity ?? 100),
          roundness: roundness ? { type: roundness.type as 1 | 2 | 3 } : null,
          ...(label?.text ? {
            label: {
              text: label.text,
              fontSize: label.fontSize || 18,
            },
          } : {}),
        } as ExcalidrawElementSkeleton,
      };
    }

    case "create_ellipse": {
      const label = args.label as LabelObject | undefined;

      return {
        skeleton: {
          type: "ellipse",
          id,
          x: Number(args.x || 100),
          y: Number(args.y || 100),
          width: Math.max(Number(args.width || 120), 120),
          height: Math.max(Number(args.height || 60), 60),
          strokeColor: String(args.strokeColor || "#1e1e1e"),
          backgroundColor: String(args.backgroundColor || "transparent"),
          fillStyle: (args.fillStyle as "solid" | "hachure" | "cross-hatch") || "solid",
          strokeWidth: Number(args.strokeWidth || 2),
          roughness: 1,
          opacity: Number(args.opacity ?? 100),
          ...(label?.text ? {
            label: {
              text: label.text,
              fontSize: label.fontSize || 18,
            },
          } : {}),
        } as ExcalidrawElementSkeleton,
      };
    }

    case "create_diamond": {
      const label = args.label as LabelObject | undefined;

      return {
        skeleton: {
          type: "diamond",
          id,
          x: Number(args.x || 100),
          y: Number(args.y || 100),
          width: Math.max(Number(args.width || 120), 120),
          height: Math.max(Number(args.height || 80), 80),
          strokeColor: String(args.strokeColor || "#1e1e1e"),
          backgroundColor: String(args.backgroundColor || "transparent"),
          fillStyle: (args.fillStyle as "solid" | "hachure" | "cross-hatch") || "solid",
          strokeWidth: Number(args.strokeWidth || 2),
          roughness: 1,
          opacity: Number(args.opacity ?? 100),
          ...(label?.text ? {
            label: {
              text: label.text,
              fontSize: label.fontSize || 16,
            },
          } : {}),
        } as ExcalidrawElementSkeleton,
      };
    }

    case "create_text": {
      const fontSize = Number(args.fontSize || 20);
      const text = String(args.text || "");

      return {
        skeleton: {
          type: "text",
          id,
          x: Number(args.x || 100),
          y: Number(args.y || 100),
          text,
          fontSize,
          strokeColor: String(args.strokeColor || "#1e1e1e"),
        } as ExcalidrawElementSkeleton,
      };
    }

    case "create_arrow": {
      const label = args.label as LabelObject | undefined;
      const startBindingArg = args.startBinding as BindingObject | undefined;
      const endBindingArg = args.endBinding as BindingObject | undefined;

      const startX = Number(args.startX || 0);
      const startY = Number(args.startY || 0);
      const endX = Number(args.endX || 100);
      const endY = Number(args.endY || 0);

      // Ensure we don't pass width/height of 0 — let points define the shape
      const dx = endX - startX;
      const dy = endY - startY;

      const skeleton: Record<string, unknown> = {
        type: "arrow",
        id,
        x: startX,
        y: startY,
        points: [[0, 0], [dx, dy]],
        strokeColor: String(args.strokeColor || "#1e1e1e"),
        strokeWidth: Number(args.strokeWidth || 2),
        strokeStyle: (args.strokeStyle as "solid" | "dashed" | "dotted") || "solid",
        roughness: 1,
        opacity: 100,
        startArrowhead: (args.startArrowhead as string) || null,
        endArrowhead: (args.endArrowhead as string) || "arrow",
      };

      if (label?.text) {
        skeleton.label = {
          text: label.text,
          fontSize: label.fontSize || 14,
        };
      }

      // Collect bindings to wire up after conversion
      const bindings: ArrowBindings = {};
      if (startBindingArg?.elementId) {
        bindings.startElementId = startBindingArg.elementId;
      }
      if (endBindingArg?.elementId) {
        bindings.endElementId = endBindingArg.elementId;
      }

      return {
        skeleton: skeleton as unknown as ExcalidrawElementSkeleton,
        bindings: (bindings.startElementId || bindings.endElementId) ? bindings : undefined,
      };
    }

    case "create_line": {
      const rawPoints = args.points as [number, number][] || [[0, 0], [100, 0]];
      const x = rawPoints[0]?.[0] || 0;
      const y = rawPoints[0]?.[1] || 0;

      return {
        skeleton: {
          type: "line",
          id,
          x,
          y,
          points: rawPoints.map((p, i) =>
            i === 0 ? [0, 0] : [p[0] - x, p[1] - y]
          ),
          strokeColor: String(args.strokeColor || "#1e1e1e"),
          strokeWidth: Number(args.strokeWidth || 2),
          strokeStyle: (args.strokeStyle as "solid" | "dashed" | "dotted") || "solid",
          roughness: 1,
          opacity: 100,
          startArrowhead: (args.startArrowhead as string) || null,
          endArrowhead: (args.endArrowhead as string) || null,
        } as unknown as ExcalidrawElementSkeleton,
      };
    }

    case "create_zone": {
      const label = args.label as LabelObject | undefined;

      return {
        skeleton: {
          type: "rectangle",
          id,
          x: Number(args.x || 0),
          y: Number(args.y || 0),
          width: Number(args.width || 800),
          height: Number(args.height || 600),
          strokeColor: String(args.strokeColor || "#b0b0b0"),
          backgroundColor: String(args.backgroundColor || "#dbe4ff"),
          fillStyle: "solid",
          strokeWidth: Number(args.strokeWidth || 1),
          roughness: 0,
          opacity: Number(args.opacity ?? 35),
          roundness: { type: 3 as const },
          ...(label?.text ? {
            label: {
              text: label.text,
              fontSize: label.fontSize || 16,
            },
          } : {}),
        } as ExcalidrawElementSkeleton,
      };
    }

    default:
      return { skeleton: null };
  }
}

// ─── Arrow Binding Helper ────────────────────────────────────────────────────
// Manually wires up arrow bindings to existing canvas elements since
// convertToExcalidrawElements can only bind within its own input array.

function applyArrowBindings(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  elements: any[],
  arrowId: string,
  bindings: ArrowBindings
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
): any[] {
  return elements.map((el) => {
    // Update the arrow's startBinding/endBinding
    if (el.id === arrowId) {
      const updates: Record<string, unknown> = {};

      if (bindings.startElementId) {
        updates.startBinding = {
          elementId: bindings.startElementId,
          focus: 0,
          gap: 5,
        };
      }

      if (bindings.endElementId) {
        updates.endBinding = {
          elementId: bindings.endElementId,
          focus: 0,
          gap: 5,
        };
      }

      return { ...el, ...updates };
    }

    // Update target elements' boundElements to include this arrow
    if (
      el.id === bindings.startElementId ||
      el.id === bindings.endElementId
    ) {
      const existingBound = el.boundElements || [];
      const alreadyBound = existingBound.some(
        (b: { id: string }) => b.id === arrowId
      );
      if (!alreadyBound) {
        return {
          ...el,
          boundElements: [...existingBound, { id: arrowId, type: "arrow" }],
        };
      }
    }

    return el;
  });
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

function applyEditText(
  api: ExcalidrawImperativeAPI,
  args: Record<string, unknown>
): void {
  const elementId = String(args.elementId || "");
  if (!elementId) return;

  const elements = api.getSceneElements();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const updated = elements.map((el: any) => {
    // Case 1: direct text element
    if (el.id === elementId && el.type === "text") {
      const patched = { ...el };
      if (args.text !== undefined) patched.text = String(args.text);
      if (args.fontSize !== undefined) patched.fontSize = Number(args.fontSize);
      if (args.strokeColor !== undefined) patched.strokeColor = String(args.strokeColor);
      return patched;
    }

    // Case 2: labeled shape — update strokeColor on the shape
    if (el.id === elementId && el.type !== "text" && args.strokeColor !== undefined) {
      return { ...el, strokeColor: String(args.strokeColor) };
    }

    return el;
  });

  // Also update bound text elements for labeled shapes
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const targetEl = elements.find((e: any) => e.id === elementId);
  if (targetEl && (targetEl as any).type !== "text") {
    const boundElements = (targetEl as any).boundElements as Array<{ id: string; type: string }> | undefined;
    if (boundElements && (args.text !== undefined || args.fontSize !== undefined)) {
      for (const bound of boundElements) {
        if (bound.type === "text") {
          const textIdx = updated.findIndex((e) => e.id === bound.id);
          if (textIdx !== -1) {
            const textEl = { ...updated[textIdx] } as any;
            if (args.text !== undefined) textEl.text = String(args.text);
            if (args.fontSize !== undefined) textEl.fontSize = Number(args.fontSize);
            if (args.strokeColor !== undefined) textEl.strokeColor = String(args.strokeColor);
            updated[textIdx] = textEl;
          }
        }
      }
    }
  }

  api.updateScene({ elements: updated });
}

// ─── Auto Layout Helper ──────────────────────────────────────────────────────
// Repositions selected elements into a clean vertical, horizontal, or grid
// layout. Sizes are preserved; only x/y are updated. When no elementIds are
// provided, all non-deleted top-level elements (not arrows or text labels
// bound to other shapes) are arranged.

function applyAutoLayout(
  api: ExcalidrawImperativeAPI,
  args: Record<string, unknown>,
): string[] {
  const layout = String(args.layout || "vertical") as "vertical" | "horizontal" | "grid";
  const spacing = Number(args.spacing ?? 40);
  const columns = Math.max(1, Number(args.columns ?? 3));
  const requestedIds = (args.elementIds as string[]) || [];

  const elements = api.getSceneElements();
  if (elements.length === 0) return [];

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const isArrangeable = (el: any): boolean => {
    if (el.isDeleted) return false;
    // skip text bound to a container — it auto-positions
    if (el.type === "text" && el.containerId) return false;
    // skip arrows — they're routed by their bindings
    if (el.type === "arrow") return false;
    return true;
  };

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const candidates = (elements as any[]).filter((el) =>
    requestedIds.length > 0 ? requestedIds.includes(el.id) && isArrangeable(el) : isArrangeable(el),
  );

  if (candidates.length === 0) return [];

  // Anchor: top-left of the current bounding box of candidates,
  // unless explicit originX/originY provided.
  const minX = Math.min(...candidates.map((e) => e.x));
  const minY = Math.min(...candidates.map((e) => e.y));
  const originX = args.originX !== undefined ? Number(args.originX) : minX;
  const originY = args.originY !== undefined ? Number(args.originY) : minY;

  const positions = new Map<string, { x: number; y: number }>();

  if (layout === "vertical") {
    let y = originY;
    for (const el of candidates) {
      positions.set(el.id, { x: originX, y });
      y += el.height + spacing;
    }
  } else if (layout === "horizontal") {
    let x = originX;
    for (const el of candidates) {
      positions.set(el.id, { x, y: originY });
      x += el.width + spacing;
    }
  } else {
    // grid: rows of `columns`. Row height = max element height in that row.
    const rowMaxHeights: number[] = [];
    for (let i = 0; i < candidates.length; i += columns) {
      const row = candidates.slice(i, i + columns);
      rowMaxHeights.push(Math.max(...row.map((e) => e.height)));
    }
    // Column widths = max width across all rows in that column slot.
    const colMaxWidths = new Array(columns).fill(0);
    candidates.forEach((el, idx) => {
      const c = idx % columns;
      if (el.width > colMaxWidths[c]) colMaxWidths[c] = el.width;
    });

    candidates.forEach((el, idx) => {
      const r = Math.floor(idx / columns);
      const c = idx % columns;
      const x =
        originX + colMaxWidths.slice(0, c).reduce((sum, w) => sum + w + spacing, 0);
      const y =
        originY + rowMaxHeights.slice(0, r).reduce((sum, h) => sum + h + spacing, 0);
      positions.set(el.id, { x, y });
    });
  }

  // Apply: shift each candidate to its new position, and shift its bound
  // text labels by the same delta so labels stay centered.
  const deltas = new Map<string, { dx: number; dy: number }>();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const updated = (elements as any[]).map((el) => {
    const pos = positions.get(el.id);
    if (pos) {
      deltas.set(el.id, { dx: pos.x - el.x, dy: pos.y - el.y });
      return { ...el, x: pos.x, y: pos.y };
    }
    return el;
  });

  // Shift bound text labels along with their container
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const finalElements = updated.map((el: any) => {
    if (el.type === "text" && el.containerId && deltas.has(el.containerId)) {
      const d = deltas.get(el.containerId)!;
      return { ...el, x: el.x + d.dx, y: el.y + d.dy };
    }
    return el;
  });

  api.updateScene({ elements: finalElements });

  // auto_layout doesn't create new elements; return [] so undo tracking is unaffected.
  return [];
}

// ─── Mermaid Converter Helper ────────────────────────────────────────────────
// Parses Mermaid syntax into Excalidraw skeletons via
// @excalidraw/mermaid-to-excalidraw, converts them, and adds them to the
// scene at the optional anchor (x, y).

async function applyConvertMermaid(
  api: ExcalidrawImperativeAPI,
  args: Record<string, unknown>,
): Promise<string[]> {
  const syntaxRaw = String(args.syntax || "").trim();
  if (!syntaxRaw) return [];

  // Strip accidental ```mermaid fences
  const syntax = syntaxRaw
    .replace(/^```\s*mermaid\s*/i, "")
    .replace(/```\s*$/i, "")
    .trim();

  const anchorX = args.x !== undefined ? Number(args.x) : 100;
  const anchorY = args.y !== undefined ? Number(args.y) : 100;

  try {
    // Lazy import to keep the main bundle smaller and load mermaid only when used
    const { parseMermaidToExcalidraw } = await import(
      "@excalidraw/mermaid-to-excalidraw"
    );

    const result = await parseMermaidToExcalidraw(syntax, {
      themeVariables: { fontSize: "16px" },
    });

    if (!result.elements || result.elements.length === 0) {
      console.warn("[AIChatPanel] Mermaid produced no elements");
      return [];
    }

    // Parser returns elements anchored at (0, 0). Shift to anchor.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const shifted = (result.elements as any[]).map((el) => ({
      ...el,
      x: (el.x ?? 0) + anchorX,
      y: (el.y ?? 0) + anchorY,
    }));

    const converted = convertToExcalidrawElements(
      shifted as ExcalidrawElementSkeleton[],
      { regenerateIds: true },
    );

    if (converted.length === 0) return [];

    const currentElements = api.getSceneElements();
    api.updateScene({
      elements: [...currentElements, ...converted],
    });

    return converted.map((e) => e.id);
  } catch (err) {
    console.error("[AIChatPanel] Mermaid parse error:", err);
    return [];
  }
}

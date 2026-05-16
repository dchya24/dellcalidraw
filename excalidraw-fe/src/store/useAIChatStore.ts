import { create } from "zustand";
import { persist } from "zustand/middleware";
import type {
  ChatMessage,
  AIConfig,
  SSEEvent,
  ToolCall,
} from "../types/ai";

// Maximum conversations to keep in localStorage (prevent storage bloat)
const MAX_CONVERSATIONS = 20;
// Maximum messages per conversation (older messages are pruned)
const MAX_MESSAGES_PER_CONVERSATION = 100;

interface AIChatStore {
  // Per-sheet chat conversations
  conversations: Record<string, ChatMessage[]>; // tabId -> messages
  activeTabId: string | null;

  // UI state
  isPanelOpen: boolean;
  isLoading: boolean;
  isStreaming: boolean;

  // Config
  aiConfig: AIConfig;
  availableModels: string[];

  // Actions
  initConversation: (tabId: string) => void;
  addMessage: (tabId: string, message: ChatMessage) => void;
  updateLastMessage: (tabId: string, messageId: string, updates: Partial<ChatMessage>) => void;
  clearConversation: (tabId: string) => void;

  // UI
  togglePanel: () => void;
  setActiveTabId: (tabId: string) => void;
  setLoading: (loading: boolean) => void;
  setStreaming: (streaming: boolean) => void;

  // Config
  setAIConfig: (config: Partial<AIConfig>) => void;
  setAvailableModels: (models: string[]) => void;

  // SSE event processing
  processEvent: (tabId: string, event: SSEEvent) => void;

  // Getters
  getConversation: (tabId: string) => ChatMessage[];
  getLastMessage: (tabId: string) => ChatMessage | null;
}

const defaultAIConfig: AIConfig = {
  provider: "openai",
  model: "gpt-4o",
  baseURL: "",
  maxTokens: 4096,
  temperature: 0.7,
};

export const useAIChatStore = create<AIChatStore>()(
  persist(
    (set, get) => ({
      conversations: {},
      activeTabId: null,
      isPanelOpen: false,
      isLoading: false,
      isStreaming: false,
      aiConfig: defaultAIConfig,
      availableModels: [],

      initConversation: (tabId: string) => {
        set((state) => {
          if (!state.conversations[tabId]) {
            return {
              conversations: {
                ...state.conversations,
                [tabId]: [
                  {
                    id: crypto.randomUUID(),
                    role: "assistant",
                    content: "Halo! Saya AI Assistant yang bisa membantu membuat diagram. Tanyakan sesuatu, misalnya \"Buatkan flowchart login\" atau \"Tambahkan flowchart proses checkout\".",
                    timestamp: Date.now(),
                  },
                ],
              },
              activeTabId: tabId,
            };
          }
          return { activeTabId: tabId };
        });
      },

      addMessage: (tabId: string, message: ChatMessage) => {
        set((state) => {
          const msgs = [...(state.conversations[tabId] || []), message];

          // Prune old messages if exceeding limit
          const pruned = msgs.length > MAX_MESSAGES_PER_CONVERSATION
            ? msgs.slice(-MAX_MESSAGES_PER_CONVERSATION)
            : msgs;

          return {
            conversations: {
              ...state.conversations,
              [tabId]: pruned,
            },
          };
        });
      },

      updateLastMessage: (tabId: string, messageId: string, updates: Partial<ChatMessage>) => {
        set((state) => {
          const messages = state.conversations[tabId] || [];
          const index = messages.findIndex((m) => m.id === messageId);
          if (index === -1) return state;

          const updated = [...messages];
          updated[index] = { ...updated[index], ...updates };
          return {
            conversations: {
              ...state.conversations,
              [tabId]: updated,
            },
          };
        });
      },

      clearConversation: (tabId: string) => {
        set((state) => ({
          conversations: {
            ...state.conversations,
            [tabId]: [
              {
                id: crypto.randomUUID(),
                role: "assistant",
                content: "Percakapan telah di-reset. Ada yang bisa saya bantu?",
                timestamp: Date.now(),
              },
            ],
          },
        }));
      },

      togglePanel: () => {
        set((state) => ({ isPanelOpen: !state.isPanelOpen }));
      },

      setActiveTabId: (tabId: string) => {
        set({ activeTabId: tabId });
      },

      setLoading: (loading: boolean) => {
        set({ isLoading: loading });
      },

      setStreaming: (streaming: boolean) => {
        set({ isStreaming: streaming });
      },

      setAIConfig: (config: Partial<AIConfig>) => {
        set((state) => ({
          aiConfig: { ...state.aiConfig, ...config },
        }));
      },

      setAvailableModels: (models: string[]) => {
        set({ availableModels: models });
      },

      processEvent: (tabId: string, event: SSEEvent) => {
        const state = get();

        switch (event.type) {
          case "text": {
            // Append or update last assistant message with streaming text
            const messages = state.conversations[tabId] || [];
            const lastMsg = messages[messages.length - 1];

            if (lastMsg && lastMsg.role === "assistant") {
              // Update existing message
              const updatedContent = lastMsg.content + event.content;
              const updated = [...messages];
              updated[messages.length - 1] = { ...lastMsg, content: updatedContent };
              set({
                conversations: {
                  ...state.conversations,
                  [tabId]: updated,
                },
              });
            }
            break;
          }

          case "tool_call": {
            // Add tool call to last assistant message or create new one
            const messages = state.conversations[tabId] || [];
            const lastMsg = messages[messages.length - 1];

            if (lastMsg && lastMsg.role === "assistant") {
              const toolCalls: ToolCall[] = [
                ...(lastMsg.toolCalls || []),
                {
                  id: event.id,
                  name: event.name,
                  arguments: event.arguments,
                },
              ];
              const updated = [...messages];
              updated[messages.length - 1] = { ...lastMsg, toolCalls };
              set({
                conversations: {
                  ...state.conversations,
                  [tabId]: updated,
                },
              });
            } else {
              // Create new assistant message with tool call
              const newMsg: ChatMessage = {
                id: crypto.randomUUID(),
                role: "assistant",
                content: "",
                timestamp: Date.now(),
                toolCalls: [
                  {
                    id: event.id,
                    name: event.name,
                    arguments: event.arguments,
                  },
                ],
              };
              set({
                conversations: {
                  ...state.conversations,
                  [tabId]: [...messages, newMsg],
                },
              });
            }
            break;
          }

          case "tool_result": {
            // Append tool result to last assistant message
            const messages = state.conversations[tabId] || [];
            const lastMsg = messages[messages.length - 1];

            if (lastMsg && lastMsg.role === "assistant") {
              const toolResults = [
                ...(lastMsg.toolResults || []),
                {
                  callId: event.callId,
                  success: event.success,
                  result: event.result,
                  error: event.error,
                },
              ];
              const updated = [...messages];
              updated[messages.length - 1] = { ...lastMsg, toolResults };
              set({
                conversations: {
                  ...state.conversations,
                  [tabId]: updated,
                },
              });
            }
            break;
          }

          case "done":
          case "error": {
            // Finalize - stop streaming state is handled by caller
            break;
          }
        }
      },

      getConversation: (tabId: string) => {
        return get().conversations[tabId] || [];
      },

      getLastMessage: (tabId: string) => {
        const messages = get().conversations[tabId] || [];
        return messages[messages.length - 1] || null;
      },
    }),
    {
      name: "ai-chat-storage",
      partialize: (state) => {
        // Prune conversations: keep only the most recent ones
        const entries = Object.entries(state.conversations);
        let pruned = state.conversations;
        if (entries.length > MAX_CONVERSATIONS) {
          // Sort by most recent message timestamp, keep top N
          const sorted = entries
            .sort(([, a], [, b]) => {
              const aTime = a[a.length - 1]?.timestamp || 0;
              const bTime = b[b.length - 1]?.timestamp || 0;
              return bTime - aTime;
            })
            .slice(0, MAX_CONVERSATIONS);
          pruned = Object.fromEntries(sorted);
        }

        return {
          conversations: pruned,
          aiConfig: {
            provider: state.aiConfig.provider,
            model: state.aiConfig.model,
            maxTokens: state.aiConfig.maxTokens,
            temperature: state.aiConfig.temperature,
            // Note: apiKey and baseURL are NOT persisted for security
          },
        };
      },
    }
  )
);
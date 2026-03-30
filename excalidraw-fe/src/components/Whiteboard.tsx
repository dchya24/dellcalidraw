import { useRef, useCallback, useState, useEffect, useMemo } from "react";
import debounce from "lodash.debounce";
import { Excalidraw } from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import type {
  AppState,
  ExcalidrawImperativeAPI,
  BinaryFileData,
  BinaryFiles,
} from "@excalidraw/excalidraw/types";
import type { OrderedExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useThemeStore } from "../store/useThemeStore";
import { roomService } from "../services/roomService";
import { elementSyncService } from "../services/elementSyncService";
import { cursorService } from "../services/cursorService";
import TabBar from "./TabBar";
import Toolbar from "./Toolbar";
import ConfirmDialog from "./ConfirmDialog";
import Sidebar from "./Sidebar";
import RemoteCursors from "./RemoteCursors";
import FloatingTab from "./FloatingTab";
import RoomInviteDialog from "./RoomInviteDialog";
import ConflictResolutionPanel from "./ConflictResolutionPanel";

interface WhiteboardProps {
  username: string;
}

export default function Whiteboard({ username }: WhiteboardProps) {
  const excalidrawAPIRef = useRef<ExcalidrawImperativeAPI | null>(null);
  const applyingRemoteChangesRef = useRef(false); // Track when applying remote changes to prevent loops
  const isApplyingChangesRef = useRef(false); // Additional lock to prevent race conditions
  const [excalidrawAPI, setExcalidrawAPI] =
    useState<ExcalidrawImperativeAPI | null>(null);
  const [isReady, setIsReady] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [pendingDeleteId, setPendingDeleteId] = useState<string | null>(null);
  const [conflictWarning, setConflictWarning] = useState<string | null>(null);
  const [floatingTabOpen, setFloatingTabOpen] = useState(false);
  const [conflicts, setConflicts] = useState<Array<{
    id: string;
    userId: string;
    username: string;
    color: string;
    timestamp: number;
    description: string;
    elementCount: number;
  }>>([]);
  const {
    files,
    activeFileId,
    getActiveFile,
    saveTabState,
    addTab,
    removeTab,
    setActiveTab,
    getActiveTabRoomId,
  } = useWhiteboardStore();
  const { theme, toggleTheme } = useThemeStore();

  const activeFile = getActiveFile();
  const activeTabId = activeFile?.activeTabId || "";
  const tabs = useMemo(() => activeFile?.tabs || [], [activeFile]);

  // Track previous active file ID to detect actual file switches
  const prevActiveFileIdRef = useRef<string | null>(null);

  // Handle switching files - save current tab and load new file's active tab
  useEffect(() => {
    const api = excalidrawAPIRef.current;
    if (!api || !isReady) return;

    // Only run when activeFileId actually changes, not on other store updates
    if (prevActiveFileIdRef.current === activeFileId) {
      return;
    }

    const prevFileId = prevActiveFileIdRef.current;
    prevActiveFileIdRef.current = activeFileId;

    // Save to the previous tab before switching
    if (prevFileId) {
      const elements = api.getSceneElements();
      const appState = api.getAppState();
      const files_data = api.getFiles();
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = appState;

      const file = files.find((f) => f.id === prevFileId);
      if (file) {
        const prevTab = file.tabs.find((t) => t.id === file.activeTabId);
        if (prevTab) {
          saveTabState(prevTab.id, elements, safeAppState, files_data);
        }
      }
    }

    // Load the new file's active tab
    if (activeFile) {
      const newTab = activeFile.tabs.find(
        (t) => t.id === activeFile.activeTabId,
      );
      if (newTab) {
        api.updateScene({
          elements: newTab.data.elements,
        });
        api.history.clear();
      }
    }
  }, [activeFileId, activeFile, files, isReady, saveTabState]); // Run when activeFileId changes

  // Handle delete with confirmation
  const handleDeleteRequest = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      const hasContent = tab && tab.data.elements.length > 0;

      if (hasContent) {
        setPendingDeleteId(tabId);
        setDeleteDialogOpen(true);
      } else {
        removeTab(tabId);
      }
    },
    [tabs, removeTab],
  );

  const handleFloatingTabOpen = useCallback(
    () => {
      setFloatingTabOpen(!floatingTabOpen);
    },
    [setFloatingTabOpen, floatingTabOpen],
  );

  const handleDeleteConfirm = useCallback(() => {
    if (pendingDeleteId) {
      removeTab(pendingDeleteId);
    }
    setDeleteDialogOpen(false);
    setPendingDeleteId(null);
  }, [pendingDeleteId, removeTab]);

  const handleDeleteCancel = useCallback(() => {
    setDeleteDialogOpen(false);
    setPendingDeleteId(null);
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      const isModPressed = e.ctrlKey || e.metaKey;

      if (isModPressed) {
        switch (e.key.toLowerCase()) {
          case "s":
            e.preventDefault();
            console.log("Manual save triggered");
            break;
          case "t":
            e.preventDefault();
            addTab();
            break;
          case "w":
            e.preventDefault();
            if (tabs.length > 1) {
              removeTab(activeTabId);
            }
            break;
          case "d":
            e.preventDefault();
            toggleTheme();
            break;
          case "tab": {
            e.preventDefault();
            const currentIndex = tabs.findIndex((t) => t.id === activeTabId);
            const nextIndex = (currentIndex + 1) % tabs.length;
            if (tabs[nextIndex]) {
              setActiveTab(tabs[nextIndex].id);
            }
            break;
          }
          case "1":
          case "2":
          case "3":
          case "4":
          case "5":
          case "6":
          case "7":
          case "8":
          case "9": {
            e.preventDefault();
            const tabIndex = parseInt(e.key) - 1;
            if (tabs[tabIndex]) {
              setActiveTab(tabs[tabIndex].id);
            }
            break;
          }
        }
      } else {
        switch (e.key) {
          case "Delete":
          case "Backspace":
            if (tabs.length > 1) {
              e.preventDefault();
              removeTab(activeTabId);
            }
            break;
          case "Escape":
            e.preventDefault();
            excalidrawAPIRef.current?.updateScene({});
            break;
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeTabId, tabs, addTab, removeTab, setActiveTab, toggleTheme]);

  // Wait for hydration
  useEffect(() => {
    const checkHydration = () => {
      if (useWhiteboardStore.persist.hasHydrated()) {
        setIsReady(true);
      }
    };

    checkHydration();

    const unsub = useWhiteboardStore.persist.onFinishHydration(() => {
      setIsReady(true);
    });

    return () => unsub();
  }, []);

  // Helper function to convert backend element to Excalidraw format
  const convertBackendToExcalidraw = useCallback(
    (backendEl: import('../types/websocket').ExcalidrawElementPayload): OrderedExcalidrawElement => {
      // Generate required fields with defaults if missing
      const seed = backendEl.seed ?? Math.floor(Math.random() * 1000000);
      const version = backendEl.version ?? 1;
      const versionNonce = backendEl.versionNonce ?? Math.floor(Math.random() * 1000000);
      const updated = backendEl.updated ?? Date.now();
      
      // Build the base element with all required properties
      const baseElement = {
        id: backendEl.id,
        type: backendEl.type as OrderedExcalidrawElement["type"],
        x: backendEl.x,
        y: backendEl.y,
        width: backendEl.width ?? 0,
        height: backendEl.height ?? 0,
        angle: backendEl.angle ?? 0,
        strokeColor: backendEl.strokeColor ?? "#000000",
        backgroundColor: backendEl.backgroundColor ?? "transparent",
        fillStyle: (backendEl.fillStyle ?? "solid") as OrderedExcalidrawElement["fillStyle"],
        strokeWidth: backendEl.strokeWidth ?? 1,
        strokeStyle: (backendEl.strokeStyle ?? "solid") as OrderedExcalidrawElement["strokeStyle"],
        roughness: backendEl.roughness ?? 1,
        opacity: backendEl.opacity ?? 100,
        seed,
        version,
        versionNonce,
        index: null, // Will be set by Excalidraw
        isDeleted: backendEl.isDeleted ?? false,
        groupIds: backendEl.groupIds ?? [],
        frameId: backendEl.frameId ?? null,
        boundElements: backendEl.boundElements ?? null,
        updated,
        link: backendEl.link ?? null,
        locked: backendEl.locked ?? false,
        // Add roundness with proper type
        roundness: null as OrderedExcalidrawElement["roundness"],
      };

      // Merge any additional data from the backend
      if (backendEl.data) {
        Object.assign(baseElement, backendEl.data);
      }

      return baseElement as OrderedExcalidrawElement;
    },
    [],
  );

  // Get initial data for Excalidraw - depends on activeFile
  const getInitialData = useCallback(() => {
    const currentFile = getActiveFile();
    if (!currentFile) return undefined;

    const activeTab = currentFile.tabs.find(
      (t) => t.id === currentFile.activeTabId,
    );

    if (activeTab && activeTab.data.elements.length > 0) {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = activeTab.data.appState || {};

      // Convert files from Record<string, unknown> to BinaryFiles format
      const binaryFiles: BinaryFiles = {};

      Object.entries(activeTab.data.files || {}).forEach(([key, value]) => {
        if (
          typeof value === 'object' &&
          value !== null &&
          'mimeType' in value &&
          'id' in value &&
          'dataURL' in value &&
          'created' in value
        ) {
          const fileValue = value as Record<string, unknown>;
          const mimeType = String(fileValue.mimeType);
          binaryFiles[key] = {
            mimeType: mimeType as BinaryFileData["mimeType"],
            id: String(fileValue.id),
            dataURL: String(fileValue.dataURL),
            created: Number(fileValue.created),
          } as BinaryFileData;
        }
      });

      return {
        elements: activeTab.data.elements,
        appState: safeAppState,
        files: binaryFiles,
      };
    }
    return undefined;
  }, [getActiveFile]);

  // Track previous tab ID for room switching
  const prevActiveTabIdRef = useRef<string | null>(null);
  const roomId = getActiveTabRoomId();

  // Initialize element sync when room changes
  useEffect(() => {
    if (!isReady || !roomId) return;

    // Only proceed if tab actually changed (avoid redundant reconnections)
    if (prevActiveTabIdRef.current === roomId) {
      return;
    }

    const prevRoomId = prevActiveTabIdRef.current;
    prevActiveTabIdRef.current = roomId;

    // Leave previous room if any
    if (prevRoomId && prevRoomId !== roomId) {
      console.log("🚪 Leaving previous room:", prevRoomId);
      roomService.leaveRoom();
    }

    console.log(
      "🔄 Initializing element sync for room:",
      roomId,
      "username:",
      username,
    );
    elementSyncService.enableSync();

    // Note: Auto-connect disabled - users must manually join via CollaborationPanel
    // roomService.joinRoom(roomId, username).catch((error) => {
    //   console.error('❌ Failed to join room:', error);
    // });

    return () => {
      // Note: We don't leave room here to avoid disconnections on re-renders
      // Room cleanup happens when switching to a different room
    };
  }, [roomId, username, isReady]);

  // Handle initial room state when joining a room
  useEffect(() => {
    const unsubscribe = roomService.onRoomState((payload) => {
      const api = excalidrawAPIRef.current;
      if (!api) return;

      console.log(
        "📦 Loading initial room state with",
        payload.elements?.length,
        "elements",
      );

      // Convert backend elements to Excalidraw format
      const elements =
        payload.elements?.map((backendEl) =>
          convertBackendToExcalidraw(backendEl),
        ) || [];

      // Set flags to prevent onChange from syncing these changes back
      applyingRemoteChangesRef.current = true;
      isApplyingChangesRef.current = true;

      // Update the scene with initial elements
      if (elements.length > 0) {
        api.updateScene({
          elements,
        });
        console.log("✅ Loaded", elements.length, "elements from room state");
      }

      // Reset flags after updateScene completes
      requestAnimationFrame(() => {
        applyingRemoteChangesRef.current = false;
        isApplyingChangesRef.current = false;
      });

      // Initialize element sync service with current elements
      elementSyncService.initializeElements(elements);
    });

    return unsubscribe;
  }, [convertBackendToExcalidraw]);

  const handleAPIReady = useCallback((api: ExcalidrawImperativeAPI) => {
    excalidrawAPIRef.current = api;
    setExcalidrawAPI(api);
  }, []);

  const handleTabChange = useCallback(
    (newTabId: string) => {
      const api = excalidrawAPIRef.current;
      if (!api) return;

      const elements = api.getSceneElements();
      const appState = api.getAppState();
      const files = api.getFiles();
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = appState;
      saveTabState(activeTabId, elements, safeAppState, files);

      const newTab = getActiveFile()?.tabs.find((t) => t.id === newTabId);
      if (newTab) {
        api.updateScene({
          elements: newTab.data.elements,
        });
        api.history.clear();
      }
    },
    [activeTabId, saveTabState, getActiveFile],
  );

  // Debounced save handler for performance optimization
  const debouncedSave = useMemo(() => {
    const saveFn = (
      tabId: string,
      elements: readonly OrderedExcalidrawElement[],
      appState: AppState,
      files: Record<string, unknown>,
    ) => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = appState;
      saveTabState(tabId, elements, safeAppState, files);
    };
    return debounce(saveFn, 500) as typeof saveFn;
  }, [saveTabState]); // 500ms debounce delay

  const handleChange = useCallback(
    (
      elements: readonly OrderedExcalidrawElement[],
      appState: AppState,
      files: Record<string, unknown>,
    ) => {
      // Skip sync if we're applying remote changes to prevent infinite loops
      if (applyingRemoteChangesRef.current || isApplyingChangesRef.current) {
        return;
      }

      // Send changes to backend for real-time sync
      elementSyncService.sendChanges(elements);

      // Save locally with debouncing
      debouncedSave(activeTabId, elements, appState, files);
    },
    [activeTabId, debouncedSave],
  );

  // Handle incoming element updates from other users
  useEffect(() => {
    const unsubscribe = elementSyncService.onElementsUpdated((payload) => {
      const api = excalidrawAPIRef.current;
      if (!api) return;

      console.log("📥 Applying remote element changes:", payload);

      // Get participant info for conflict tracking
      const participants = roomService.getParticipants();
      const participant = participants.find(p => p.id === payload.userId);
      const username = participant?.username || "Unknown";
      const color = participant?.color || "#888888";

      // Count elements changed
      const addedCount = payload.changes.added?.length || 0;
      const updatedCount = payload.changes.updated?.length || 0;
      const deletedCount = payload.changes.deleted?.length || 0;
      const totalChanged = addedCount + updatedCount + deletedCount;

      // Show conflict warning toast
      if (payload.userId) {
        setConflictWarning(`${username} just made changes (${totalChanged} elements)`);
        setTimeout(() => setConflictWarning(null), 3000);

        // Add to conflicts list
        const conflictId = `${payload.userId}-${Date.now()}`;
        const changeDescriptions: string[] = [];
        if (addedCount > 0) changeDescriptions.push(`added ${addedCount}`);
        if (updatedCount > 0) changeDescriptions.push(`updated ${updatedCount}`);
        if (deletedCount > 0) changeDescriptions.push(`deleted ${deletedCount}`);

        setConflicts(prev => {
          // Keep only last 10 conflicts to prevent UI overflow
          const newConflicts = [
            {
              id: conflictId,
              userId: payload.userId,
              username,
              color,
              timestamp: Date.now(),
              description: changeDescriptions.join(", ") || "made changes",
              elementCount: totalChanged,
            },
            ...prev,
          ].slice(0, 10);
          return newConflicts;
        });
      }

      // Get current elements
      const currentElements = api.getSceneElements();
      const elementMap = new Map(currentElements.map((el) => [el.id, el]));

      // Apply changes
      if (payload.changes.added) {
        payload.changes.added.forEach((backendEl) => {
          const excalidrawEl = convertBackendToExcalidraw(backendEl);
          elementMap.set(backendEl.id, excalidrawEl);
        });
      }

      if (payload.changes.updated) {
        payload.changes.updated.forEach((backendEl) => {
          const excalidrawEl = convertBackendToExcalidraw(backendEl);
          elementMap.set(backendEl.id, excalidrawEl);
        });
      }

      if (payload.changes.deleted) {
        payload.changes.deleted.forEach((id) => {
          elementMap.delete(id);
        });
      }

      // Set flags to prevent onChange from syncing these changes back
      applyingRemoteChangesRef.current = true;
      isApplyingChangesRef.current = true;

      // Update the scene with merged elements
      api.updateScene({
        elements: Array.from(elementMap.values()),
      });

      // Reset flags after updateScene completes
      requestAnimationFrame(() => {
        applyingRemoteChangesRef.current = false;
        isApplyingChangesRef.current = false;
      });

      // Save to local storage
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = api.getAppState();
      saveTabState(
        activeTabId,
        Array.from(elementMap.values()),
        safeAppState,
        api.getFiles(),
      );
    });

    return unsubscribe;
  }, [activeTabId, saveTabState, convertBackendToExcalidraw]);

  const handleAddTab = useCallback(() => {
    const api = excalidrawAPIRef.current;
    if (!api) return;

    const elements = api.getSceneElements();
    const appState = api.getAppState();
    const files = api.getFiles();
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { collaborators, ...safeAppState } = appState;
    saveTabState(activeTabId, elements, safeAppState, files);

    addTab();

    api.updateScene({ elements: [] });
    api.history.clear();
  }, [activeTabId, saveTabState, addTab]);

  // Drag and drop import handler
  const handleDrop = useCallback(
    async (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const files = Array.from(e.dataTransfer.files);
      const jsonFile = files.find(
        (f) =>
          f.name.endsWith(".excalidraw") ||
          f.name.endsWith(".json") ||
          f.type === "application/json",
      );

      if (jsonFile) {
        try {
          const text = await jsonFile.text();
          const data = JSON.parse(text);

          // Handle both single tab and multi-file formats
          if (data.type === "excalidraw" && data.elements) {
            // Single tab format (native Excalidraw)
            const api = excalidrawAPIRef.current;
            if (api) {
              // Save current state first
              const currentElements = api.getSceneElements();
              const currentAppState = api.getAppState();
              const currentFiles = api.getFiles();
              // eslint-disable-next-line @typescript-eslint/no-unused-vars
              const { collaborators, ...safeAppState } = currentAppState;
              saveTabState(
                activeTabId,
                currentElements,
                safeAppState,
                currentFiles,
              );

              // Create new tab with imported data
              addTab();

              api.updateScene({
                elements: data.elements || [],
                appState: data.appState || {},
              });
              api.history.clear();
            }
          } else if (data.tabs && Array.isArray(data.tabs)) {
            // Multi-tab format (our custom format)
            const { loadFromFile } = useWhiteboardStore.getState();
            loadFromFile({
              tabs: data.tabs,
              activeTabId: data.activeTabId || data.tabs[0]?.id,
            });
          }
        } catch (error) {
          console.error("Failed to import file:", error);
          alert(
            "Failed to import file. Please make sure it is a valid .excalidraw or .json file.",
          );
        }
      }
    },
    [activeTabId, saveTabState, addTab],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  // Initialize cursor tracking when Excalidraw is ready
  const [mousePosition, setMousePosition] = useState({ x: 0, y: 0 });

  // Track mouse position for cursor sync
  useEffect(() => {
    const container = document.querySelector('.excalidraw-container');
    if (!container) return;

    const handleMouseMove = (e: MouseEvent) => {
      setMousePosition({ x: e.clientX, y: e.clientY });
    };

    container.addEventListener('mousemove', handleMouseMove);
    return () => container.removeEventListener('mousemove', handleMouseMove);
  }, []);

  useEffect(() => {
    if (!excalidrawAPI || !isReady || !roomId) return;

    // Start cursor tracking with proper coordinate transformation
    cursorService.startTracking(() => {
      const appState = excalidrawAPI.getAppState();
      const zoom = appState.zoom?.value || 1;
      
      // Transform screen coordinates to canvas coordinates
      // This is what we send to other users
      const canvasX = (mousePosition.x - appState.offsetLeft - appState.scrollX) / zoom;
      const canvasY = (mousePosition.y - appState.offsetTop - appState.scrollY) / zoom;
      
      return {
        x: canvasX,
        y: canvasY,
      };
    });

    return () => {
      cursorService.stopTracking();
    };
  }, [excalidrawAPI, isReady, activeTabId, roomId]);

  if (!isReady) {
    return (
      <div className="w-screen h-screen flex items-center justify-center">
        <div>Loading...</div>
      </div>
    );
  }

  return (
    <div
      className={`w-screen h-screen flex flex-col ${theme === "dark" ? "dark" : ""}`}
      onDrop={handleDrop}
      onDragOver={handleDragOver}
    >
      <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />

      <div className="flex-1 relative z-0">
        <Toolbar
          excalidrawAPI={excalidrawAPI}
          onToggleSidebar={() => setSidebarOpen(!sidebarOpen)}
          username={username}
        />

        {
          floatingTabOpen && (
            <FloatingTab
              tabs={tabs}
              activeTabId={activeTabId}
              onTabChange={handleTabChange}
              onDeleteRequest={handleDeleteRequest}
            />
          )
        }

        {/* Sync Status Indicator */}
        {conflictWarning && (
          <div className="absolute top-16 left-1/2 transform -translate-x-1/2 z-50 px-4 py-2 bg-yellow-500 text-white rounded-lg shadow-lg flex items-center gap-2">
            <svg
              className="w-5 h-5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            {conflictWarning}
          </div>
        )}

        <Excalidraw
          key={`${activeFileId}-${activeTabId}`} // Force re-render when file or tab changes
          excalidrawAPI={handleAPIReady}
          onChange={handleChange}
          initialData={getInitialData()}
        />
        <RemoteCursors excalidrawAPI={excalidrawAPI} />

        {/* Conflict Resolution Panel */}
        <ConflictResolutionPanel
          conflicts={conflicts}
          onDismiss={(id) => setConflicts(prev => prev.filter(c => c.id !== id))}
          onDismissAll={() => setConflicts([])}
        />

        {/* Room Invite Dialog */}
        <RoomInviteDialog username={username} />

      </div>
      <TabBar
        onTabChange={handleTabChange}
        onAddTab={handleAddTab}
        onDeleteRequest={handleDeleteRequest}
        handleFloatingTabOpen={handleFloatingTabOpen}
      />
      <ConfirmDialog
        isOpen={deleteDialogOpen}
        title="Delete Sheet"
        message="This sheet contains elements. Are you sure you want to delete it? This action cannot be undone."
        confirmLabel="Delete"
        cancelLabel="Cancel"
        onConfirm={handleDeleteConfirm}
        onCancel={handleDeleteCancel}
      />
    </div>
  );
}

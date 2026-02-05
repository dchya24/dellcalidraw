import { useRef, useCallback, useState, useEffect, useMemo } from "react";
import debounce from "lodash.debounce";
import { Excalidraw } from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import type { AppState, ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
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
import { OrderedExcalidrawElement } from "@excalidraw/excalidraw";

interface WhiteboardProps {
  username: string;
}

export default function Whiteboard({ username }: WhiteboardProps) {
  const excalidrawAPIRef = useRef<ExcalidrawImperativeAPI | null>(null);
  const [excalidrawAPI, setExcalidrawAPI] =
    useState<ExcalidrawImperativeAPI | null>(null);
  const [isReady, setIsReady] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [pendingDeleteId, setPendingDeleteId] = useState<string | null>(null);
  const [syncStatus, setSyncStatus] = useState<'synced' | 'syncing' | 'error'>('synced');
  const [conflictWarning, setConflictWarning] = useState<string | null>(null);
  const { files, activeFileId, getActiveFile, saveTabState, addTab, removeTab, setActiveTab, getActiveTabRoomId } = useWhiteboardStore();
  const { theme, toggleTheme } = useThemeStore();

  const activeFile = getActiveFile();
  const activeTabId = activeFile?.activeTabId || "";
  const tabs = useMemo(() => activeFile?.tabs || [], [activeFile]);

  // Handle switching files - save current tab and load new file's active tab
  useEffect(() => {
    const api = excalidrawAPIRef.current;
    if (!api || !isReady) return;

    // Get the current state from canvas before switching
    const elements = api.getSceneElements();
    const appState = api.getAppState();
    const files_data = api.getFiles();
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { collaborators, ...safeAppState } = appState;

    // Save to the previous tab
    const file = files.find(f => f.id === activeFileId);
    if (file) {
      const prevTab = file.tabs.find(t => t.id === file.activeTabId);
      if (prevTab) {
        saveTabState(prevTab.id, elements, safeAppState, files_data);
      }
    }

    // Load the new file's active tab
    if (activeFile) {
      const newTab = activeFile.tabs.find(t => t.id === activeFile.activeTabId);
      if (newTab) {
        api.updateScene({
          elements: newTab.data.elements,
        });
        api.history.clear();
      }
    }
  }, [activeFileId, activeFile, files, isReady, saveTabState]); // Run when activeFileId changes

  // Handle delete with confirmation
  const handleDeleteRequest = useCallback((tabId: string) => {
    const tab = tabs.find((t) => t.id === tabId);
    const hasContent = tab && tab.data.elements.length > 0;

    if (hasContent) {
      setPendingDeleteId(tabId);
      setDeleteDialogOpen(true);
    } else {
      removeTab(tabId);
    }
  }, [tabs, removeTab]);

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
          case "z":
            e.preventDefault();
            excalidrawAPIRef.current?.history.undo();
            break;
          case "y":
            e.preventDefault();
            excalidrawAPIRef.current?.history.redo();
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
  // Define this early so it can be used in other callbacks
  const convertBackendToExcalidraw = useCallback((backendEl: {
    id: string;
    type: string;
    x: number;
    y: number;
    width: number;
    height: number;
    angle: number;
    stroke: string;
    background: string;
    fill: string;
    data?: {any: unknown};
  }) => {
    return {
      id: backendEl.id,
      type: backendEl.type,
      x: backendEl.x,
      y: backendEl.y,
      width: backendEl.width,
      height: backendEl.height,
      angle: backendEl.angle,
      strokeColor: backendEl.stroke,
      backgroundColor: backendEl.background,
      fillStyle: backendEl.fill,
      ...(backendEl.data || {}),
    };
  }, []);

  // Get initial data for Excalidraw - depends on activeFile
  const getInitialData = useCallback(() => {
    const currentFile = getActiveFile();
    if (!currentFile) return undefined;

    const activeTab = currentFile.tabs.find((t) => t.id === currentFile.activeTabId);

    if (activeTab && activeTab.data.elements.length > 0) {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = activeTab.data.appState || {};
      return {
        elements: activeTab.data.elements,
        appState: safeAppState,
        files: activeTab.data.files,
      };
    }
    return undefined;
  }, [getActiveFile]);

  // Initialize element sync when room changes
  useEffect(() => {
    const roomId = getActiveTabRoomId();
    if (roomId) {
      console.log('🔄 Initializing element sync for room:', roomId);
      elementSyncService.enableSync();

      // Join the room
      roomService.joinRoom(roomId, username).catch((error) => {
        console.error('Failed to join room:', error);
      });

      return () => {
        elementSyncService.disableSync();
      };
    }
  }, [activeTabId, username, getActiveTabRoomId]);

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
    [activeTabId, saveTabState, getActiveFile]
  );

  // Debounced save handler for performance optimization
  const debouncedSave = useMemo(
    () => debounce((tabId: string, elements: readonly unknown[], appState: AppState, files: Record<string, unknown>) => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = appState;
      saveTabState(tabId, elements as readonly unknown[], safeAppState, files);
    }, 500), // 500ms debounce delay
    [saveTabState]
  );

  const handleChange = useCallback(
    (elements: readonly OrderedExcalidrawElement[], appState: AppState, files: Record<string, unknown>) => {
      // Send changes to backend for real-time sync
      setSyncStatus('syncing');
      elementSyncService.sendChanges(elements);

      // Save locally with debouncing
      debouncedSave(activeTabId, elements, appState, files);

      // Reset sync status after a short delay
      setTimeout(() => setSyncStatus('synced'), 300);
    },
    [activeTabId, debouncedSave]
  );

  // Handle incoming element updates from other users
  useEffect(() => {
    const unsubscribe = elementSyncService.onElementsUpdated((payload) => {
      const api = excalidrawAPIRef.current;
      if (!api) return;

      console.log('📥 Applying remote element changes:', payload);

      // Show conflict warning if someone else is editing
      if (payload.userId) {
        setConflictWarning(`Another user just made changes`);
        setTimeout(() => setConflictWarning(null), 3000);
      }

      // Get current elements
      const currentElements = api.getSceneElements();
      const elementMap = new Map(currentElements.map(el => [el.id, el]));

      // Apply changes
      if (payload.changes.added) {
        payload.changes.added.forEach(backendEl => {
          const excalidrawEl = convertBackendToExcalidraw(backendEl);
          elementMap.set(backendEl.id, excalidrawEl);
        });
      }

      if (payload.changes.updated) {
        payload.changes.updated.forEach(backendEl => {
          const excalidrawEl = convertBackendToExcalidraw(backendEl);
          elementMap.set(backendEl.id, excalidrawEl);
        });
      }

      if (payload.changes.deleted) {
        payload.changes.deleted.forEach(id => {
          elementMap.delete(id);
        });
      }

      // Update the scene with merged elements
      api.updateScene({
        elements: Array.from(elementMap.values()),
      });

      // Save to local storage
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { collaborators, ...safeAppState } = api.getAppState();
      saveTabState(activeTabId, Array.from(elementMap.values()), safeAppState, api.getFiles());
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
  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();

    const files = Array.from(e.dataTransfer.files);
    const jsonFile = files.find(f =>
      f.name.endsWith('.excalidraw') ||
      f.name.endsWith('.json') ||
      f.type === 'application/json'
    );

    if (jsonFile) {
      try {
        const text = await jsonFile.text();
        const data = JSON.parse(text);

        // Handle both single tab and multi-file formats
        if (data.type === 'excalidraw' && data.elements) {
          // Single tab format (native Excalidraw)
          const api = excalidrawAPIRef.current;
          if (api) {
            // Save current state first
            const currentElements = api.getSceneElements();
            const currentAppState = api.getAppState();
            const currentFiles = api.getFiles();
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { collaborators, ...safeAppState } = currentAppState;
            saveTabState(activeTabId, currentElements, safeAppState, currentFiles);

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
            activeTabId: data.activeTabId || data.tabs[0]?.id
          });
        }
      } catch (error) {
        console.error('Failed to import file:', error);
        alert('Failed to import file. Please make sure it is a valid .excalidraw or .json file.');
      }
    }
  }, [activeTabId, saveTabState, addTab]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  // Initialize cursor tracking when Excalidraw is ready
  useEffect(() => {
    if (!excalidrawAPI || !isReady) return;

    const roomId = getActiveTabRoomId();
    if (!roomId) return;

    // Start cursor tracking
    cursorService.startTracking(() => {
      const appState = excalidrawAPI.getAppState();
      return {
        x: appState.scrollX || 0,
        y: appState.scrollY || 0,
      };
    });

    return () => {
      cursorService.stopTracking();
    };
  }, [excalidrawAPI, isReady, activeTabId, getActiveTabRoomId]);

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

        {/* Sync Status Indicator */}
        {conflictWarning && (
          <div className="absolute top-16 left-1/2 transform -translate-x-1/2 z-50 px-4 py-2 bg-yellow-500 text-white rounded-lg shadow-lg flex items-center gap-2">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            {conflictWarning}
          </div>
        )}

        {/* Sync status dot */}
        <div className="absolute top-20 right-4 z-50 flex items-center gap-2 px-3 py-1.5 rounded-full bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm shadow-sm">
          <div className={`w-2 h-2 rounded-full ${
            syncStatus === 'synced' ? 'bg-green-500' :
            syncStatus === 'syncing' ? 'bg-yellow-500 animate-pulse' :
            'bg-red-500'
          }`} />
          <span className="text-xs text-gray-600 dark:text-gray-400">
            {syncStatus === 'synced' ? 'Synced' :
             syncStatus === 'syncing' ? 'Syncing...' :
             'Sync Error'}
          </span>
        </div>

        <Excalidraw
          key={`${activeFileId}-${activeTabId}`} // Force re-render when file or tab changes
          excalidrawAPI={handleAPIReady}
          onChange={handleChange}
          initialData={getInitialData()}
        />
        <RemoteCursors />
      </div>
      <TabBar
        onTabChange={handleTabChange}
        onAddTab={handleAddTab}
        onDeleteRequest={handleDeleteRequest}
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

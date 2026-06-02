import { create } from "zustand";
import { persist } from "zustand/middleware";
import { nanoid } from "nanoid";
import type { AppState } from "@excalidraw/excalidraw/types";
import type { OrderedExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import { fileService, type UserFile } from "../services/fileService";
import { useAuthStore } from "./useAuthStore";

export interface WhiteboardTab {
  id: string;
  title: string;
  roomId: string;
  data: {
    elements: readonly OrderedExcalidrawElement[];
    appState: Partial<AppState>;
    files: Record<string, unknown>;
  };
  lastModified: number;
}

export interface WhiteboardFile {
  id: string;
  name: string;
  tabs: WhiteboardTab[];
  activeTabId: string;
  createdAt: number;
  lastModified: number;
  isCloud: boolean;
  cloudId?: string;
}

interface AppStore {
  files: WhiteboardFile[];
  activeFileId: string;
  isLoading: boolean;
  syncStatus: "idle" | "syncing" | "synced" | "error";
  lastSyncedAt: number | null;
  /**
   * Local guest files awaiting a user decision (merge to cloud or discard).
   * Set by `loadFiles` when the user logs in and the cloud account already
   * has files. The UI is expected to render a dialog and call
   * `confirmMigration` / `discardMigration`.
   */
  pendingMigration: WhiteboardFile[] | null;

  // File operations
  createFile: (name?: string) => Promise<void>;
  deleteFile: (id: string) => Promise<void>;
  renameFile: (id: string, newName: string) => Promise<void>;
  duplicateFile: (id: string) => void;
  setActiveFile: (id: string) => void;

  // Tab operations (within active file)
  addTab: () => void;
  removeTab: (tabId: string) => void;
  renameTab: (tabId: string, newTitle: string) => void;
  setActiveTab: (tabId: string) => void;
  saveTabState: (
    tabId: string,
    elements: readonly OrderedExcalidrawElement[],
    appState: Partial<AppState>,
    files: Record<string, unknown>
  ) => void;
  regenerateRoomId: (tabId: string) => void;

  // Cloud sync operations
  loadFiles: () => Promise<void>;
  syncToCloud: (file: WhiteboardFile) => Promise<void>;
  confirmMigration: () => Promise<void>;
  discardMigration: () => void;
  resetToGuestState: () => void;

  // Getters
  getActiveFile: () => WhiteboardFile | undefined;
  getActiveTab: () => WhiteboardTab | undefined;
  getActiveTabRoomId: () => string;

  // Import/Export (per file)
  loadFromFile: (data: { tabs: WhiteboardTab[]; activeTabId: string }) => void;
  loadNativeExcalidraw: (
    elements: readonly OrderedExcalidrawElement[],
    appState: Partial<AppState>,
    files: Record<string, unknown>
  ) => void;
  getExportData: () => { tabs: WhiteboardTab[]; activeTabId: string };
}

const createEmptyTab = (title: string, id?: string): WhiteboardTab => ({
  id: id || nanoid(),
  title,
  roomId: nanoid(10),
  data: {
    elements: [],
    appState: {},
    files: {},
  },
  lastModified: Date.now(),
});

const createEmptyFile = (name?: string, id?: string): WhiteboardFile => {
  const initialTabId = nanoid();
  return {
    id: id || nanoid(),
    name: name || "Untitled",
    tabs: [createEmptyTab("Sheet 1", initialTabId)],
    activeTabId: initialTabId,
    createdAt: Date.now(),
    lastModified: Date.now(),
    isCloud: false,
  };
};

const INITIAL_FILE_ID = "initial-file";
const initialFile = createEmptyFile("Untitled", INITIAL_FILE_ID);

export const useWhiteboardStore = create<AppStore>()(
  persist(
    (set, get) => ({
      files: [initialFile],
      activeFileId: INITIAL_FILE_ID,
      isLoading: false,
      syncStatus: "idle" as const,
      lastSyncedAt: null,
      pendingMigration: null,

      // File operations
      createFile: async (name?: string) => {
        const authState = useAuthStore.getState();
        const { files } = get();
        const fileName = name || `Untitled ${files.length + 1}`;

        if (authState.isAuthenticated) {
          try {
            const response = await fileService.createFile(fileName);
            const cloudFile: WhiteboardFile = {
              id: response.file.id,
              name: response.file.name,
              tabs: [createEmptyTab("Sheet 1")],
              activeTabId: "",
              createdAt: new Date(response.file.createdAt).getTime(),
              lastModified: Date.now(),
              isCloud: true,
              cloudId: response.file.id,
            };
            cloudFile.activeTabId = cloudFile.tabs[0].id;
            set({
              files: [...get().files, cloudFile],
              activeFileId: cloudFile.id,
            });
          } catch (error) {
            console.error("Failed to create cloud file:", error);
            // Fallback to local
            const newFile = createEmptyFile(fileName);
            set({
              files: [...get().files, newFile],
              activeFileId: newFile.id,
            });
          }
        } else {
          const newFile = createEmptyFile(fileName);
          set({
            files: [...get().files, newFile],
            activeFileId: newFile.id,
          });
        }
      },

      deleteFile: async (id: string) => {
        const { files, activeFileId } = get();
        if (files.length <= 1) return;

        const file = files.find((f) => f.id === id);
        if (!file) return;

        const authState = useAuthStore.getState();
        if (authState.isAuthenticated && file.isCloud && file.cloudId) {
          try {
            await fileService.deleteFile(file.cloudId);
          } catch (error) {
            console.error("Failed to delete cloud file:", error);
          }
        }

        const newFiles = files.filter((f) => f.id !== id);
        const newActiveFileId =
          activeFileId === id ? newFiles[0].id : activeFileId;

        set({ files: newFiles, activeFileId: newActiveFileId });
      },

      renameFile: async (id: string, newName: string) => {
        const file = get().files.find((f) => f.id === id);
        if (!file) return;

        const authState = useAuthStore.getState();
        if (authState.isAuthenticated && file.isCloud && file.cloudId) {
          try {
            await fileService.renameFile(file.cloudId, newName);
          } catch (error) {
            console.error("Failed to rename cloud file:", error);
          }
        }

        set((state) => ({
          files: state.files.map((f) =>
            f.id === id ? { ...f, name: newName, lastModified: Date.now() } : f
          ),
        }));
      },

      setActiveFile: (id: string) => {
        set({ activeFileId: id });
      },

      duplicateFile: (id: string) => {
        const { files } = get();
        const file = files.find((f) => f.id === id);
        if (!file) return;

        // Deep clone tabs with new IDs
        const newTabs = file.tabs.map((tab) => ({
          ...tab,
          id: nanoid(),
          roomId: nanoid(10),
          data: {
            elements: [...tab.data.elements],
            appState: { ...tab.data.appState },
            files: { ...tab.data.files },
          },
          lastModified: Date.now(),
        }));

        const newFile: WhiteboardFile = {
          id: nanoid(),
          name: `${file.name} (copy)`,
          tabs: newTabs,
          activeTabId: newTabs[0].id,
          createdAt: Date.now(),
          lastModified: Date.now(),
          isCloud: false,
        };

        set({
          files: [...get().files, newFile],
          activeFileId: newFile.id,
        });
      },

      // Tab operations (within active file)
      addTab: () => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            const newTab = createEmptyTab(`Sheet ${f.tabs.length + 1}`);
            return {
              ...f,
              tabs: [...f.tabs, newTab],
              activeTabId: newTab.id,
              lastModified: Date.now(),
            };
          }),
        }));
      },

      removeTab: (tabId: string) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            if (f.tabs.length <= 1) return f;

            const newTabs = f.tabs.filter((t) => t.id !== tabId);
            const newActiveTabId =
              f.activeTabId === tabId ? newTabs.at(-1)!.id : f.activeTabId;

            return {
              ...f,
              tabs: newTabs,
              activeTabId: newActiveTabId,
              lastModified: Date.now(),
            };
          }),
        }));
      },

      renameTab: (tabId: string, newTitle: string) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            return {
              ...f,
              tabs: f.tabs.map((tab) =>
                tab.id === tabId ? { ...tab, title: newTitle } : tab
              ),
              lastModified: Date.now(),
            };
          }),
        }));
      },

      setActiveTab: (tabId: string) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            return { ...f, activeTabId: tabId };
          }),
        }));
      },

      saveTabState: (
        tabId: string,
        elements: readonly OrderedExcalidrawElement[],
        appState: Partial<AppState>,
        files: Record<string, unknown>
      ) => {
        const { activeFileId } = get();
        set((state) => {
          const fileIndex = state.files.findIndex((f) => f.id === activeFileId);
          if (fileIndex === -1) return state;

          const file = state.files[fileIndex];
          const tabIndex = file.tabs.findIndex((tab) => tab.id === tabId);
          if (tabIndex === -1) return state;

          const currentTab = file.tabs[tabIndex];

          // Check if data actually changed to prevent unnecessary updates
          const elementsChanged =
            JSON.stringify(currentTab.data.elements) !==
            JSON.stringify(elements);
          const appStateChanged =
            JSON.stringify(currentTab.data.appState) !==
            JSON.stringify(appState);
          const filesChanged =
            JSON.stringify(currentTab.data.files) !== JSON.stringify(files);

          if (!elementsChanged && !appStateChanged && !filesChanged) {
            return state;
          }

          const newFiles = [...state.files];
          newFiles[fileIndex] = {
            ...file,
            tabs: file.tabs.map((tab) =>
              tab.id === tabId
                ? {
                    ...tab,
                    data: { elements, appState, files },
                    lastModified: Date.now(),
                  }
                : tab
            ),
            lastModified: Date.now(),
          };

          return { files: newFiles };
        });
      },

      regenerateRoomId: (tabId: string) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            return {
              ...f,
              tabs: f.tabs.map((tab) =>
                tab.id === tabId ? { ...tab, roomId: nanoid(10) } : tab
              ),
              lastModified: Date.now(),
            };
          }),
        }));
      },

      // Cloud sync operations
      loadFiles: async () => {
        const authState = useAuthStore.getState();
        if (!authState.isAuthenticated) return;

        set({ isLoading: true, syncStatus: "syncing" as const });

        try {
          // 1. Capture current local-only files before fetching cloud data
          const currentFiles = get().files;
          const localOnlyFiles = currentFiles.filter((f) => !f.isCloud);

          // 2. Fetch cloud files
          const response = await fileService.listFiles();
          const cloudFiles: WhiteboardFile[] = response.files.map(
            (file: UserFile) => {
              // If cloud file has tabs data, reconstruct them
              const tabs =
                file.tabs && file.tabs.length > 0
                  ? file.tabs.map((t) => ({
                      id: nanoid(),
                      title: t.title,
                      roomId: t.roomId || nanoid(10),
                      data: {
                        elements: (t.elements ||
                          []) as readonly OrderedExcalidrawElement[],
                        appState: (t.appState || {}) as Partial<AppState>,
                        files: (t.files || {}) as Record<string, unknown>,
                      },
                      lastModified: Date.now(),
                    }))
                  : [createEmptyTab("Sheet 1")];

              return {
                id: file.id,
                name: file.name,
                tabs,
                activeTabId: tabs[0].id,
                createdAt: new Date(file.createdAt).getTime(),
                lastModified: new Date(file.updatedAt).getTime(),
                isCloud: true,
                cloudId: file.id,
              };
            }
          );

          // 3. Decide what to do based on cloud + local state.
          //
          //  - cloud empty + local present  -> first-time/new account, auto-migrate.
          //  - cloud present + local present -> ask user (Merge or Discard).
          //  - cloud present + no local      -> just use cloud.
          //  - both empty                    -> create one empty file.
          if (cloudFiles.length === 0 && localOnlyFiles.length > 0) {
            set({
              files: localOnlyFiles,
              activeFileId: localOnlyFiles[0].id,
              isLoading: false,
              syncStatus: "syncing" as const,
              pendingMigration: null,
            });
            await migrateLocalFiles(localOnlyFiles);
          } else if (cloudFiles.length > 0 && localOnlyFiles.length > 0) {
            // Show cloud files only; stash local files for the dialog decision.
            set({
              files: cloudFiles,
              activeFileId: cloudFiles[0].id,
              isLoading: false,
              syncStatus: "synced" as const,
              lastSyncedAt: Date.now(),
              pendingMigration: localOnlyFiles,
            });
          } else {
            const finalFiles =
              cloudFiles.length > 0 ? cloudFiles : [createEmptyFile("Untitled")];
            set({
              files: finalFiles,
              activeFileId: finalFiles[0].id,
              isLoading: false,
              syncStatus: "synced" as const,
              lastSyncedAt: Date.now(),
              pendingMigration: null,
            });
          }
        } catch (error) {
          console.error("Failed to sync from cloud:", error);
          // Keep existing data as fallback
          set({ isLoading: false, syncStatus: "error" as const });
        }
      },

      confirmMigration: async () => {
        const { pendingMigration, files } = get();
        if (!pendingMigration || pendingMigration.length === 0) {
          set({ pendingMigration: null });
          return;
        }

        // Add local files back into the visible list and migrate them.
        set({
          files: [...files, ...pendingMigration],
          pendingMigration: null,
          syncStatus: "syncing" as const,
        });
        await migrateLocalFiles(pendingMigration);
      },

      discardMigration: () => {
        set({ pendingMigration: null });
      },

      resetToGuestState: () => {
        // Strip every cloud-backed file from local storage. Keep any
        // remaining local-only files so the user does not lose unsaved guest
        // work that was created during the authenticated session.
        const { files, activeFileId } = get();
        const localFiles = files.filter((f) => !f.isCloud);

        if (localFiles.length === 0) {
          const fresh = createEmptyFile("Untitled");
          set({
            files: [fresh],
            activeFileId: fresh.id,
            isLoading: false,
            syncStatus: "idle" as const,
            lastSyncedAt: null,
            pendingMigration: null,
          });
          return;
        }

        const stillThere = localFiles.find((f) => f.id === activeFileId);
        set({
          files: localFiles,
          activeFileId: stillThere ? stillThere.id : localFiles[0].id,
          isLoading: false,
          syncStatus: "idle" as const,
          lastSyncedAt: null,
          pendingMigration: null,
        });
      },

      syncToCloud: async (file: WhiteboardFile) => {
        const authState = useAuthStore.getState();
        if (!authState.isAuthenticated) return;

        try {
          await fileService.updateFile(file.cloudId || file.id, {
            name: file.name,
            tabCount: file.tabs.length,
          });
        } catch (error) {
          console.error("Failed to sync to cloud:", error);
        }
      },

      // Getters
      getActiveFile: () => {
        const { files, activeFileId } = get();
        return files.find((f) => f.id === activeFileId);
      },

      getActiveTab: () => {
        const activeFile = get().getActiveFile();
        if (!activeFile) return undefined;
        return activeFile.tabs.find((t) => t.id === activeFile.activeTabId);
      },

      getActiveTabRoomId: () => {
        const activeTab = get().getActiveTab();
        return activeTab?.roomId || "";
      },

      // Import/Export (operates on active file)
      loadFromFile: (data: { tabs: WhiteboardTab[]; activeTabId: string }) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            return {
              ...f,
              tabs: data.tabs,
              activeTabId: data.activeTabId,
              lastModified: Date.now(),
            };
          }),
        }));
      },

      loadNativeExcalidraw: (
        elements: readonly OrderedExcalidrawElement[],
        appState: Partial<AppState>,
        files: Record<string, unknown>
      ) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            return {
              ...f,
              tabs: f.tabs.map((tab) =>
                tab.id === f.activeTabId
                  ? {
                      ...tab,
                      data: { elements, appState, files },
                      lastModified: Date.now(),
                    }
                  : tab
              ),
              lastModified: Date.now(),
            };
          }),
        }));
      },

      getExportData: () => {
        const activeFile = get().getActiveFile();
        if (!activeFile) return { tabs: [], activeTabId: "" };
        return {
          tabs: activeFile.tabs,
          activeTabId: activeFile.activeTabId,
        };
      },
    }),
    {
      name: "whiteboard-storage",
      partialize: (state) => ({
        files: state.files,
        activeFileId: state.activeFileId,
        lastSyncedAt: state.lastSyncedAt,
      }),
    }
  )
);

// Auto-sync when auth state changes
useAuthStore.subscribe((state, prevState) => {
  if (state.isAuthenticated === prevState.isAuthenticated) return;

  if (state.isAuthenticated) {
    useWhiteboardStore.getState().loadFiles();
  } else {
    // Manual logout or token-expired auto-logout. Drop cloud-backed files
    // from local storage so a guest session cannot keep operating on
    // someone else's data without a valid token.
    useWhiteboardStore.getState().resetToGuestState();
  }
});

// Helper: migrate local-only files to cloud after login
async function migrateLocalFiles(localFiles: WhiteboardFile[]) {
  try {
    const migrationPayload = localFiles.map(f => ({
      name: f.name,
      activeTabId: f.activeTabId,
      tabs: f.tabs.map(t => ({
        title: t.title,
        roomId: t.roomId,
        elements: [...t.data.elements] as unknown[],
        appState: t.data.appState as Record<string, unknown>,
        files: t.data.files as Record<string, unknown>,
      })),
    }));

    const result = await fileService.migrateFiles(migrationPayload);

    // After successful migration, update local state to mark files as cloud
    const state = useWhiteboardStore.getState();
    const updatedFiles = state.files.map(f => {
      if (f.isCloud) return f;

      // Match by name + tab count for reliability
      const migratedFile = result.files.find(mf =>
        mf.name === f.name && mf.tabs.length === f.tabs.length
      );
      if (!migratedFile) return f;

      // Reconstruct tabs with cloud data
      const tabs = migratedFile.tabs.map((t, idx) => ({
        ...(f.tabs[idx] || createEmptyTab(t.title)),
        roomId: t.roomId || f.tabs[idx]?.roomId || nanoid(10),
      }));

      return {
        ...f,
        id: migratedFile.id,
        isCloud: true,
        cloudId: migratedFile.id,
        tabs,
        activeTabId: f.activeTabId,
      };
    });

    useWhiteboardStore.setState({
      files: updatedFiles,
      syncStatus: "synced" as const,
      lastSyncedAt: Date.now(),
    });
  } catch (error) {
    console.error("Failed to migrate local files to cloud:", error);
    // Files stay as local-only, still usable
  }
}

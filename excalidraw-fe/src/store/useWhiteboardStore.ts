import { create } from "zustand";
import { persist } from "zustand/middleware";
import { nanoid } from "nanoid";

export interface WhiteboardTab {
  id: string;
  title: string;
  roomId: string;
  data: {
    elements: readonly any[];
    appState: Partial<any>;
    files: any;
  };
  lastModified: number;
}

interface WhiteboardFile {
  id: string;
  name: string;
  tabs: WhiteboardTab[];
  activeTabId: string;
  createdAt: number;
  lastModified: number;
}

interface AppStore {
  files: WhiteboardFile[];
  activeFileId: string;

  // File operations
  createFile: (name?: string) => void;
  deleteFile: (id: string) => void;
  renameFile: (id: string, newName: string) => void;
  setActiveFile: (id: string) => void;

  // Tab operations (within active file)
  addTab: () => void;
  removeTab: (tabId: string) => void;
  renameTab: (tabId: string, newTitle: string) => void;
  setActiveTab: (tabId: string) => void;
  saveTabState: (tabId: string, elements: any, appState: any, files: any) => void;
  regenerateRoomId: (tabId: string) => void;

  // Getters
  getActiveFile: () => WhiteboardFile | undefined;
  getActiveTab: () => WhiteboardTab | undefined;
  getActiveTabRoomId: () => string;

  // Import/Export (per file)
  loadFromFile: (data: { tabs: WhiteboardTab[]; activeTabId: string }) => void;
  loadNativeExcalidraw: (elements: any, appState: any, files: any) => void;
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
  };
};

const INITIAL_FILE_ID = nanoid();

export const useWhiteboardStore = create<AppStore>()(
  persist(
    (set, get) => ({
      files: [createEmptyFile("Untitled", INITIAL_FILE_ID)],
      activeFileId: INITIAL_FILE_ID,

      // File operations
      createFile: (name?: string) => {
        const { files } = get();
        const newFile = createEmptyFile(name || `Untitled ${files.length + 1}`);
        set({
          files: [...files, newFile],
          activeFileId: newFile.id,
        });
      },

      deleteFile: (id: string) => {
        const { files, activeFileId } = get();
        if (files.length <= 1) return; // Always keep at least one file

        const newFiles = files.filter((f) => f.id !== id);
        const newActiveFileId = activeFileId === id ? newFiles[0].id : activeFileId;

        set({
          files: newFiles,
          activeFileId: newActiveFileId,
        });
      },

      renameFile: (id: string, newName: string) => {
        set((state) => ({
          files: state.files.map((f) =>
            f.id === id ? { ...f, name: newName, lastModified: Date.now() } : f
          ),
        }));
      },

      setActiveFile: (id: string) => {
        set({ activeFileId: id });
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
            if (f.tabs.length <= 1) return f; // Always keep at least one tab

            const newTabs = f.tabs.filter((t) => t.id !== tabId);
            const newActiveTabId = f.activeTabId === tabId ? newTabs.at(-1)!.id : f.activeTabId;

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

      saveTabState: (tabId: string, elements: any, appState: any, files: any) => {
        const { activeFileId } = get();
        set((state) => {
          const fileIndex = state.files.findIndex((f) => f.id === activeFileId);
          if (fileIndex === -1) return state;

          const file = state.files[fileIndex];
          const tabIndex = file.tabs.findIndex((tab) => tab.id === tabId);
          if (tabIndex === -1) return state;

          const currentTab = file.tabs[tabIndex];

          // Check if data actually changed to prevent unnecessary updates
          const currentElements = currentTab.data.elements;
          const currentAppState = currentTab.data.appState;
          const currentFiles = currentTab.data.files;

          // Compare elements and appState deeply to avoid unnecessary updates
          const elementsChanged = JSON.stringify(currentElements) !== JSON.stringify(elements);
          const appStateChanged = JSON.stringify(currentAppState) !== JSON.stringify(appState);
          const filesChanged = JSON.stringify(currentFiles) !== JSON.stringify(files);

          if (!elementsChanged && !appStateChanged && !filesChanged) {
            return state; // No change, skip update to prevent infinite loop
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

      loadNativeExcalidraw: (elements: any, appState: any, files: any) => {
        const { activeFileId } = get();
        set((state) => ({
          files: state.files.map((f) => {
            if (f.id !== activeFileId) return f;
            // Load native Excalidraw data into the active tab
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
      }),
    }
  )
);

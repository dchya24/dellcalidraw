import { create } from "zustand";
import { persist } from "zustand/middleware";
import { fileService, type UserFile } from "../services/fileService";
import { useAuthStore } from "./useAuthStore";
import type { WhiteboardTab } from "./useWhiteboardStore";

// Local file representation (matches existing structure)
export interface LocalFile {
  id: string;
  name: string;
  tabs: WhiteboardTab[];
  activeTabId: string;
  createdAt: number;
  lastModified: number;
  isCloud: boolean;
  cloudId?: string;
}

// File store interface
interface FileStore {
  // State
  localFiles: LocalFile[];
  activeFileId: string;
  isLoading: boolean;
  syncStatus: "idle" | "syncing" | "synced" | "error";
  lastSyncedAt: number | null;
  
  // Local file operations (used when not authenticated)
  createLocalFile: (name?: string) => LocalFile;
  deleteLocalFile: (id: string) => void;
  renameLocalFile: (id: string, newName: string) => void;
  setActiveLocalFile: (id: string) => void;
  
  // Cloud sync operations (used when authenticated)
  syncFromCloud: () => Promise<void>;
  syncToCloud: (file: LocalFile) => Promise<void>;
  createCloudFile: (name: string) => Promise<LocalFile>;
  deleteCloudFile: (cloudId: string) => Promise<void>;
  renameCloudFile: (cloudId: string, newName: string) => Promise<void>;
  
  // Unified operations (auto-select local/cloud based on auth)
  loadFiles: () => Promise<void>;
  createFile: (name?: string) => Promise<void>;
  deleteFile: (id: string) => Promise<void>;
  renameFile: (id: string, newName: string) => Promise<void>;
  setActiveFile: (id: string) => void;
  
  // Getters
  getActiveFile: () => LocalFile | undefined;
  getFiles: () => LocalFile[];
  isAuthenticated: () => boolean;
}

// Initial file ID
const INITIAL_FILE_ID = "initial-file";

const createEmptyTab = (title: string): WhiteboardTab => ({
  id: crypto.randomUUID(),
  title,
  roomId: crypto.randomUUID().replace(/-/g, "").substring(0, 10),
  data: {
    elements: [],
    appState: {},
    files: {},
  },
  lastModified: Date.now(),
});

const createLocalEmptyFile = (name?: string, id?: string): LocalFile => {
  const initialTab = createEmptyTab("Sheet 1");
  return {
    id: id || crypto.randomUUID(),
    name: name || "Untitled",
    tabs: [initialTab],
    activeTabId: initialTab.id,
    createdAt: Date.now(),
    lastModified: Date.now(),
    isCloud: false,
  };
};

// Initial file
const initialFile = createLocalEmptyFile("Untitled", INITIAL_FILE_ID);

export const useFileStore = create<FileStore>()(
  persist(
    (set, get) => ({
      localFiles: [initialFile],
      activeFileId: INITIAL_FILE_ID,
      isLoading: false,
      syncStatus: "idle" as const,
      lastSyncedAt: null,

      // Local file operations
      createLocalFile: (name?: string) => {
        const { localFiles } = get();
        const newFile = createLocalEmptyFile(name || `Untitled ${localFiles.length + 1}`);
        set({ localFiles: [...localFiles, newFile] });
        return newFile;
      },

      deleteLocalFile: (id: string) => {
        const { localFiles, activeFileId } = get();
        if (localFiles.length <= 1) return;

        const newFiles = localFiles.filter((f) => f.id !== id);
        const newActiveFileId = activeFileId === id 
          ? (newFiles[0]?.id || INITIAL_FILE_ID) 
          : activeFileId;

        set({ localFiles: newFiles, activeFileId: newActiveFileId });
      },

      renameLocalFile: (id: string, newName: string) => {
        set((state) => ({
          localFiles: state.localFiles.map((f) =>
            f.id === id ? { ...f, name: newName, lastModified: Date.now() } : f
          ),
        }));
      },

      setActiveLocalFile: (id: string) => {
        set({ activeFileId: id });
      },

      // Cloud sync operations
      syncFromCloud: async () => {
        const authState = useAuthStore.getState();
        if (!authState.isAuthenticated) return;

        set({ isLoading: true, syncStatus: "syncing" as const });
        
        try {
          const response = await fileService.listFiles();
          const cloudFiles: LocalFile[] = response.files.map((file: UserFile) => ({
            id: file.id,
            name: file.name,
            tabs: [createEmptyTab("Sheet 1")],
            activeTabId: "",
            createdAt: new Date(file.createdAt).getTime(),
            lastModified: new Date(file.updatedAt).getTime(),
            isCloud: true,
            cloudId: file.id,
          }));

          // Set activeTabId for each cloud file
          cloudFiles.forEach(f => {
            f.activeTabId = f.tabs[0].id;
          });

          set({ 
            localFiles: cloudFiles, 
            isLoading: false, 
            syncStatus: "synced" as const,
            lastSyncedAt: Date.now() 
          });
        } catch (error) {
          console.error("Failed to sync from cloud:", error);
          set({ isLoading: false, syncStatus: "error" as const });
        }
      },

      syncToCloud: async (file: LocalFile) => {
        const authState = useAuthStore.getState();
        if (!authState.isAuthenticated) return;

        try {
          await fileService.updateFile(file.id, {
            name: file.name,
            tabCount: file.tabs.length,
          });
        } catch (error) {
          console.error("Failed to sync to cloud:", error);
        }
      },

      createCloudFile: async (name: string) => {
        const response = await fileService.createFile(name);
        const cloudFile: LocalFile = {
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
        return cloudFile;
      },

      deleteCloudFile: async (cloudId: string) => {
        await fileService.deleteFile(cloudId);
      },

      renameCloudFile: async (cloudId: string, newName: string) => {
        await fileService.renameFile(cloudId, newName);
      },

      // Unified operations
      loadFiles: async () => {
        const authState = useAuthStore.getState();
        
        if (authState.isAuthenticated) {
          await get().syncFromCloud();
        }
        // If not authenticated, files are already loaded from localStorage via persist
      },

      createFile: async (name?: string) => {
        const authState = useAuthStore.getState();

        if (authState.isAuthenticated) {
          const cloudFile = await get().createCloudFile(name || "Untitled");
          set({ localFiles: [...get().localFiles, cloudFile] });
          get().setActiveLocalFile(cloudFile.id);
        } else {
          const newFile = get().createLocalFile(name);
          get().setActiveLocalFile(newFile.id);
        }
      },

      deleteFile: async (id: string) => {
        const authState = useAuthStore.getState();
        const file = get().localFiles.find(f => f.id === id);
        
        if (!file) return;

        if (authState.isAuthenticated && file.isCloud && file.cloudId) {
          await get().deleteCloudFile(file.cloudId);
        }
        
        get().deleteLocalFile(id);
      },

      renameFile: async (id: string, newName: string) => {
        const authState = useAuthStore.getState();
        const file = get().localFiles.find(f => f.id === id);
        
        if (!file) return;

        if (authState.isAuthenticated && file.isCloud && file.cloudId) {
          await get().renameCloudFile(file.cloudId, newName);
        }
        
        get().renameLocalFile(id, newName);
      },

      setActiveFile: (id: string) => {
        set({ activeFileId: id });
      },

      // Getters
      getActiveFile: () => {
        const { localFiles, activeFileId } = get();
        return localFiles.find(f => f.id === activeFileId);
      },

      getFiles: () => get().localFiles,

      isAuthenticated: () => {
        return useAuthStore.getState().isAuthenticated;
      },
    }),
    {
      name: "file-storage",
      partialize: (state) => ({
        localFiles: state.localFiles,
        activeFileId: state.activeFileId,
        lastSyncedAt: state.lastSyncedAt,
      }),
    }
  )
);

// Auto-sync when auth state changes
useAuthStore.subscribe((state, prevState) => {
  if (state.isAuthenticated !== prevState.isAuthenticated) {
    if (state.isAuthenticated) {
      // Sync from cloud when logging in
      useFileStore.getState().loadFiles();
    }
    // When logging out, keep local files (they stay in localStorage)
  }
});
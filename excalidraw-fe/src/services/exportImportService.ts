import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import { exportToSvg, exportToBlob } from "@excalidraw/excalidraw";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import type { WhiteboardTab } from "../store/useWhiteboardStore";

// ─── Types ───────────────────────────────────────────────────────────────────

export interface DellcalidrawFileFormat {
  type: "dellcalidraw";
  version: 1;
  name: string;
  tabs: WhiteboardTab[];
  activeTabId: string;
  exportedAt: string;
}

export interface ExcalidrawNativeFormat {
  type: "excalidraw";
  version: 2;
  source: string;
  elements: readonly unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
}

export type ImportResult =
  | { format: "dellcalidraw"; data: DellcalidrawFileFormat }
  | { format: "excalidraw"; data: ExcalidrawNativeFormat }
  | { format: "elements"; data: unknown[] }
  | { format: "unknown"; data: null };

// ─── Export Functions ────────────────────────────────────────────────────────

/**
 * Export active sheet as .excalidraw JSON (single scene)
 */
export function exportActiveSheetJSON(api: ExcalidrawImperativeAPI): void {
  const elements = api.getSceneElements();
  const appState = api.getAppState();
  const files = api.getFiles();

  const exportData: ExcalidrawNativeFormat = {
    type: "excalidraw",
    version: 2,
    source: "dellcalidraw",
    elements,
    appState: {
      viewBackgroundColor: appState.viewBackgroundColor,
      gridSize: appState.gridSize,
    },
    files,
  };

  const activeTab = useWhiteboardStore.getState().getActiveTab();
  const filename = `${activeTab?.title || "whiteboard"}.excalidraw`;

  downloadJSON(exportData, filename);
}

/**
 * Export active file with all sheets as .dellcalidraw JSON
 * Saves current scene state before exporting.
 */
export function exportFileAllSheets(api: ExcalidrawImperativeAPI): void {
  const store = useWhiteboardStore.getState();
  const activeFile = store.getActiveFile();
  if (!activeFile) return;

  // Save current scene to active tab before export
  const elements = api.getSceneElements();
  const appState = api.getAppState();
  const files = api.getFiles();
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { collaborators, ...safeAppState } = appState;
  store.saveTabState(activeFile.activeTabId, elements, safeAppState, files);

  // Re-read file after save to get latest data
  const updatedFile = useWhiteboardStore.getState().getActiveFile();
  if (!updatedFile) return;

  const exportData: DellcalidrawFileFormat = {
    type: "dellcalidraw",
    version: 1,
    name: updatedFile.name,
    tabs: updatedFile.tabs,
    activeTabId: updatedFile.activeTabId,
    exportedAt: new Date().toISOString(),
  };

  const filename = `${updatedFile.name}.dellcalidraw`;
  downloadJSON(exportData, filename);
}

/**
 * Export active sheet as PNG
 */
export async function exportActiveSheetPNG(
  api: ExcalidrawImperativeAPI
): Promise<void> {
  const elements = api.getSceneElements();
  const appState = api.getAppState();
  const files = api.getFiles();

  const blob = await exportToBlob({
    elements,
    appState: { ...appState, exportWithDarkMode: false },
    files,
    mimeType: "image/png",
  });

  const activeTab = useWhiteboardStore.getState().getActiveTab();
  const filename = `${activeTab?.title || "whiteboard"}.png`;

  downloadBlob(blob, filename);
}

/**
 * Export active sheet as SVG
 */
export async function exportActiveSheetSVG(
  api: ExcalidrawImperativeAPI
): Promise<void> {
  const elements = api.getSceneElements();
  const appState = api.getAppState();
  const files = api.getFiles();

  const svg = await exportToSvg({
    elements,
    appState: { ...appState, exportWithDarkMode: false },
    files,
  });

  const svgString = new XMLSerializer().serializeToString(svg);
  const blob = new Blob([svgString], { type: "image/svg+xml" });

  const activeTab = useWhiteboardStore.getState().getActiveTab();
  const filename = `${activeTab?.title || "whiteboard"}.svg`;

  downloadBlob(blob, filename);
}

// ─── Import Functions ────────────────────────────────────────────────────────

/**
 * Parse imported file content and detect format
 */
export function parseImportData(content: string): ImportResult {
  try {
    const data = JSON.parse(content);

    // Multi-sheet dellcalidraw format
    if (data.type === "dellcalidraw" && data.tabs && data.activeTabId) {
      return { format: "dellcalidraw", data };
    }

    // Legacy multi-tab format (from old export)
    if (data.tabs && data.activeTabId && !data.type) {
      return {
        format: "dellcalidraw",
        data: {
          type: "dellcalidraw",
          version: 1,
          name: data.name || "Imported File",
          tabs: data.tabs,
          activeTabId: data.activeTabId,
          exportedAt: new Date().toISOString(),
        },
      };
    }

    // Native Excalidraw format
    if (data.type === "excalidraw" || (data.elements && !data.tabs)) {
      return { format: "excalidraw", data };
    }

    // Raw elements array
    if (Array.isArray(data)) {
      return { format: "elements", data };
    }

    return { format: "unknown", data: null };
  } catch {
    return { format: "unknown", data: null };
  }
}

/**
 * Import a dellcalidraw file (all sheets) into the active file.
 * Replaces all tabs in the active file.
 */
export function importDellcalidrawFile(
  data: DellcalidrawFileFormat,
  api: ExcalidrawImperativeAPI
): void {
  const store = useWhiteboardStore.getState();

  store.loadFromFile({
    tabs: data.tabs,
    activeTabId: data.activeTabId,
  });

  // Load active tab into Excalidraw canvas
  const activeTab = data.tabs.find((t) => t.id === data.activeTabId);
  if (activeTab) {
    api.updateScene({
      elements: activeTab.data.elements,
    });
    api.history.clear();
  }
}

/**
 * Import a dellcalidraw file as a new file (doesn't replace active file).
 */
export async function importDellcalidrawAsNewFile(
  data: DellcalidrawFileFormat,
  api: ExcalidrawImperativeAPI
): Promise<void> {
  const store = useWhiteboardStore.getState();

  // Create new file first
  await store.createFile(data.name || "Imported File");

  // Now load tabs into the newly created (now active) file
  store.loadFromFile({
    tabs: data.tabs,
    activeTabId: data.activeTabId,
  });

  // Load active tab into canvas
  const activeTab = data.tabs.find((t) => t.id === data.activeTabId);
  if (activeTab) {
    api.updateScene({
      elements: activeTab.data.elements,
    });
    api.history.clear();
  }
}

/**
 * Import native Excalidraw format into active sheet
 */
export function importExcalidrawNative(
  data: ExcalidrawNativeFormat,
  api: ExcalidrawImperativeAPI
): void {
  const store = useWhiteboardStore.getState();
  const elements = data.elements || [];
  const appState = data.appState || {};
  const files = data.files || {};

  store.loadNativeExcalidraw(
    elements as Parameters<typeof store.loadNativeExcalidraw>[0],
    appState,
    files
  );

  api.updateScene({ elements: elements as Parameters<typeof api.updateScene>[0]["elements"] });
  api.history.clear();
}

/**
 * Import raw elements array into active sheet
 */
export function importElementsArray(
  elements: unknown[],
  api: ExcalidrawImperativeAPI
): void {
  const store = useWhiteboardStore.getState();

  store.loadNativeExcalidraw(
    elements as Parameters<typeof store.loadNativeExcalidraw>[0],
    {},
    {}
  );

  api.updateScene({ elements: elements as Parameters<typeof api.updateScene>[0]["elements"] });
  api.history.clear();
}

/**
 * Handle file import from input event (auto-detect format).
 * Returns true if import was successful.
 */
export function handleFileImport(
  content: string,
  api: ExcalidrawImperativeAPI
): boolean {
  const result = parseImportData(content);

  switch (result.format) {
    case "dellcalidraw":
      importDellcalidrawFile(result.data, api);
      return true;

    case "excalidraw":
      importExcalidrawNative(result.data, api);
      return true;

    case "elements":
      importElementsArray(result.data, api);
      return true;

    case "unknown":
      return false;
  }
}

// ─── Cloud Save/Load ─────────────────────────────────────────────────────────

/**
 * Save active sheet to cloud (by roomId)
 */
export async function saveActiveSheetToCloud(
  apiService: { saveCanvas: (roomId: string) => Promise<{ count: number }> }
): Promise<{ success: boolean; count?: number; error?: string }> {
  const store = useWhiteboardStore.getState();
  const roomId = store.getActiveTabRoomId();

  if (!roomId) {
    return { success: false, error: "No active room" };
  }

  try {
    const result = await apiService.saveCanvas(roomId);
    return { success: true, count: result.count };
  } catch (err) {
    console.error("Failed to save canvas:", err);
    return { success: false, error: (err as Error).message };
  }
}

/**
 * Load active sheet from cloud (by roomId)
 */
export async function loadActiveSheetFromCloud(
  api: ExcalidrawImperativeAPI,
  apiService: { loadCanvas: (roomId: string) => Promise<{ elements: unknown[]; count: number }> }
): Promise<{ success: boolean; count?: number; error?: string }> {
  const store = useWhiteboardStore.getState();
  const roomId = store.getActiveTabRoomId();

  if (!roomId) {
    return { success: false, error: "No active room" };
  }

  try {
    const result = await apiService.loadCanvas(roomId);
    if (result.elements && result.elements.length > 0) {
      api.updateScene({
        elements: result.elements as Parameters<typeof api.updateScene>[0]["elements"],
      });
      api.history.clear();
      return { success: true, count: result.count };
    }
    return { success: false, error: "No saved canvas found" };
  } catch (err) {
    console.error("Failed to load canvas:", err);
    return { success: false, error: (err as Error).message };
  }
}

/**
 * Save all sheets in active file to cloud (loop per room)
 */
export async function saveAllSheetsToCloud(
  api: ExcalidrawImperativeAPI,
  apiService: { saveCanvas: (roomId: string) => Promise<{ count: number }> }
): Promise<{ success: boolean; savedCount: number; errors: string[] }> {
  const store = useWhiteboardStore.getState();
  const activeFile = store.getActiveFile();

  if (!activeFile) {
    return { success: false, savedCount: 0, errors: ["No active file"] };
  }

  // Save current scene to active tab first
  const elements = api.getSceneElements();
  const appState = api.getAppState();
  const files = api.getFiles();
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { collaborators, ...safeAppState } = appState;
  store.saveTabState(activeFile.activeTabId, elements, safeAppState, files);

  const errors: string[] = [];
  let savedCount = 0;

  for (const tab of activeFile.tabs) {
    try {
      await apiService.saveCanvas(tab.roomId);
      savedCount++;
    } catch (err) {
      errors.push(`${tab.title}: ${(err as Error).message}`);
    }
  }

  return {
    success: errors.length === 0,
    savedCount,
    errors,
  };
}

/**
 * Load all sheets in active file from cloud (loop per room)
 */
export async function loadAllSheetsFromCloud(
  api: ExcalidrawImperativeAPI,
  apiService: { loadCanvas: (roomId: string) => Promise<{ elements: unknown[]; count: number }> }
): Promise<{ success: boolean; loadedCount: number; errors: string[] }> {
  const store = useWhiteboardStore.getState();
  const activeFile = store.getActiveFile();

  if (!activeFile) {
    return { success: false, loadedCount: 0, errors: ["No active file"] };
  }

  const errors: string[] = [];
  let loadedCount = 0;

  for (const tab of activeFile.tabs) {
    try {
      const result = await apiService.loadCanvas(tab.roomId);
      if (result.elements && result.elements.length > 0) {
        store.saveTabState(
          tab.id,
          result.elements as Parameters<typeof store.saveTabState>[1],
          {},
          {}
        );
        loadedCount++;
      }
    } catch (err) {
      errors.push(`${tab.title}: ${(err as Error).message}`);
    }
  }

  // Reload active tab into canvas
  const updatedFile = useWhiteboardStore.getState().getActiveFile();
  if (updatedFile) {
    const activeTab = updatedFile.tabs.find(
      (t) => t.id === updatedFile.activeTabId
    );
    if (activeTab && activeTab.data.elements.length > 0) {
      api.updateScene({
        elements: activeTab.data.elements,
      });
      api.history.clear();
    }
  }

  return {
    success: errors.length === 0,
    loadedCount,
    errors,
  };
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function downloadJSON(data: unknown, filename: string): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], {
    type: "application/json",
  });
  downloadBlob(blob, filename);
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

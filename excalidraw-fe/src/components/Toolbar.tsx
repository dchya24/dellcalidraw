import { useRef, useEffect, useState } from "react";
import {
  Download,
  Upload,
  FileJson,
  Image,
  FileType,
  Moon,
  Sun,
  Sidebar,
  Save,
  FolderOpen,
  Loader2,
} from "lucide-react";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useThemeStore } from "../store/useThemeStore";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import { exportToSvg, exportToBlob } from "@excalidraw/excalidraw";
import CollaborationPanel from "./CollaborationPanel";
import type { WhiteboardTab } from "../store/useWhiteboardStore";
import { apiService } from "../services/api";
import type { OrderedExcalidrawElement } from "@excalidraw/excalidraw/element/types";

interface ToolbarProps {
  excalidrawAPI: ExcalidrawImperativeAPI | null;
  onToggleSidebar?: () => void;
  username?: string;
  isAuthenticated?: boolean;
  onOpenRoomSettings?: () => void;
}

export default function Toolbar({ excalidrawAPI, onToggleSidebar, username = "Guest", isAuthenticated = false, onOpenRoomSettings }: ToolbarProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { theme, toggleTheme } = useThemeStore();
  const {
    loadFromFile,
    loadNativeExcalidraw,
    getActiveTab,
    getActiveTabRoomId,
    regenerateRoomId,
  } = useWhiteboardStore();

  const roomId = getActiveTabRoomId();
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [saveStatus, setSaveStatus] = useState<string | null>(null);

  // Sync Excalidraw theme with app theme
  useEffect(() => {
    if (excalidrawAPI) {
      excalidrawAPI.updateScene({
        appState: {
          theme: theme,
        },
      });
    }
  }, [theme, excalidrawAPI]);

  // Clear save status after 3 seconds
  useEffect(() => {
    if (saveStatus) {
      const timer = setTimeout(() => setSaveStatus(null), 3000);
      return () => clearTimeout(timer);
    }
  }, [saveStatus]);

  // Save canvas to database
  const handleSaveToCloud = async () => {
    if (!roomId || isSaving) return;

    setIsSaving(true);
    setSaveStatus(null);

    try {
      const result = await apiService.saveCanvas(roomId);
      setSaveStatus(`Saved ${result.count} elements`);
    } catch (err) {
      console.error("Failed to save canvas:", err);
      setSaveStatus("Save failed");
    } finally {
      setIsSaving(false);
    }
  };

  // Load canvas from database
  const handleLoadFromCloud = async () => {
    if (!roomId || !excalidrawAPI || isLoading) return;

    setIsLoading(true);
    setSaveStatus(null);

    try {
      const result = await apiService.loadCanvas(roomId);
      if (result.elements && result.elements.length > 0) {
        excalidrawAPI.updateScene({
          elements: result.elements as OrderedExcalidrawElement[],
        });
        excalidrawAPI.history.clear();
        setSaveStatus(`Loaded ${result.count} elements`);
      } else {
        setSaveStatus("No saved canvas found");
      }
    } catch (err) {
      console.error("Failed to load canvas:", err);
      setSaveStatus("Load failed");
    } finally {
      setIsLoading(false);
    }
  };

  // Export in native Excalidraw format (current tab only)
  const handleExportJSON = () => {
    if (!excalidrawAPI) return;

    const elements = excalidrawAPI.getSceneElements();
    const appState = excalidrawAPI.getAppState();
    const files = excalidrawAPI.getFiles();

    const exportData = {
      type: "excalidraw",
      version: 2,
      source: "whiteboard-app",
      elements: elements,
      appState: {
        viewBackgroundColor: appState.viewBackgroundColor,
        gridSize: appState.gridSize,
      },
      files: files,
    };

    const blob = new Blob([JSON.stringify(exportData, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const activeTab = getActiveTab();
    a.download = `${activeTab?.title || "whiteboard"}.excalidraw`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleExportPNG = async () => {
    if (!excalidrawAPI) return;
    const elements = excalidrawAPI.getSceneElements();
    const appState = excalidrawAPI.getAppState();
    const files = excalidrawAPI.getFiles();

    const blob = await exportToBlob({
      elements,
      appState: { ...appState, exportWithDarkMode: false },
      files,
      mimeType: "image/png",
    });

    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const activeTab = getActiveTab();
    a.download = `${activeTab?.title || "whiteboard"}.png`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleExportSVG = async () => {
    if (!excalidrawAPI) return;
    const elements = excalidrawAPI.getSceneElements();
    const appState = excalidrawAPI.getAppState();
    const files = excalidrawAPI.getFiles();

    const svg = await exportToSvg({
      elements,
      appState: { ...appState, exportWithDarkMode: false },
      files,
    });

    const svgString = new XMLSerializer().serializeToString(svg);
    const blob = new Blob([svgString], { type: "image/svg+xml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const activeTab = getActiveTab();
    a.download = `${activeTab?.title || "whiteboard"}.svg`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleImport = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        const data = JSON.parse(event.target?.result as string);

        // Check if it's our multi-tab format
        if (data.tabs && data.activeTabId) {
          loadFromFile(data);
          if (excalidrawAPI) {
            const activeTab = data.tabs.find(
              (t: WhiteboardTab) => t.id === data.activeTabId
            );
            if (activeTab) {
              excalidrawAPI.updateScene({
                elements: activeTab.data.elements,
              });
              excalidrawAPI.history.clear();
            }
          }
        }
        // Check if it's native Excalidraw format
        else if (Array.isArray(data.elements) || data.type === "excalidraw") {
          const elements = data.elements || [];
          const appState = data.appState || {};
          const files = data.files || {};

          loadNativeExcalidraw(elements, appState, files);

          if (excalidrawAPI) {
            excalidrawAPI.updateScene({ elements });
            excalidrawAPI.history.clear();
          }
        }
        // Maybe it's just an array of elements
        else if (Array.isArray(data)) {
          loadNativeExcalidraw(data, {}, {});

          if (excalidrawAPI) {
            excalidrawAPI.updateScene({ elements: data });
            excalidrawAPI.history.clear();
          }
        }
        else {
          alert("Unrecognized file format");
        }
      } catch (err) {
        console.error("Failed to parse file:", err);
        alert("Invalid file format: " + (err as Error).message);
      }
    };
    reader.readAsText(file);
    e.target.value = "";
  };

  return (
    <div className="absolute bottom-2 right-1/2 translate-x-1/2 z-10">
      <div className={`flex items-center gap-1 shadow-lg border px-1 transition-colors rounded-xl ${
        theme === "dark"
          ? "bg-gray-800/95 border-gray-700 backdrop-blur-sm"
          : "bg-white/95 border-gray-200 backdrop-blur-sm"
      }`}>
        {/* Sidebar Toggle */}
        <button
          onClick={onToggleSidebar}
          className={`pl-2 rounded-lg transition-colors ${
            theme === "dark" ? "hover:bg-gray-700 text-gray-300" : "hover:bg-gray-100 text-gray-600"
          }`}
          title="Toggle sidebar"
        >
          <Sidebar size={18} />
        </button>

        {/* Collaboration Section */}
        <CollaborationPanel
          roomId={roomId}
          username={username}
          isAuthenticated={isAuthenticated}
          onRegenerateRoomId={() => regenerateRoomId(roomId)}
          onOpenSettings={onOpenRoomSettings}
        />

        {/* Import/Export Section */}
        <div className="flex items-center gap-2">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileChange}
            accept=".excalidraw,.json"
            className="hidden"
          />
          <button
            onClick={handleImport}
            className={`py-2 rounded-lg transition-colors ${
              theme === "dark" ? "hover:bg-gray-700 text-gray-300" : "hover:bg-gray-100 text-gray-600"
            }`}
            title="Import file"
          >
            <Download size={18} />
          </button>
          <div className="relative group">
            <button
              className={`py-2 rounded-lg transition-colors ${
                theme === "dark" ? "hover:bg-gray-700 text-gray-300" : "hover:bg-gray-100 text-gray-600"
              }`}
              title="Export"
            >
              <Upload size={18} />
            </button>
            <div className={`absolute right-0 bottom-full mt-2 rounded-lg shadow-xl border opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all min-w-45 overflow-hidden ${
              theme === "dark" ? "bg-gray-800 border-gray-700" : "bg-white border-gray-200"
            }`}>
              <button
                onClick={handleExportJSON}
                className={`flex items-center gap-3 px-4 py-2.5 w-full text-left text-sm border-b ${
                  theme === "dark" ? "hover:bg-gray-700 text-gray-200 border-gray-700" : "hover:bg-gray-100 text-gray-700 border-gray-100"
                }`}
              >
                <FileJson size={16} />
                <span>Export JSON</span>
              </button>
              <button
                onClick={handleExportPNG}
                className={`flex items-center gap-3 px-4 py-2.5 w-full text-left text-sm border-b ${
                  theme === "dark" ? "hover:bg-gray-700 text-gray-200 border-gray-700" : "hover:bg-gray-100 text-gray-700 border-gray-100"
                }`}
              >
                <Image size={16} />
                <span>Export PNG</span>
              </button>
              <button
                onClick={handleExportSVG}
                className={`flex items-center gap-3 px-4 py-2.5 w-full text-left text-sm ${
                  theme === "dark" ? "hover:bg-gray-700 text-gray-200" : "hover:bg-gray-100 text-gray-700"
                }`}
              >
                <FileType size={16} />
                <span>Export SVG</span>
              </button>
            </div>
          </div>
        </div>

        {/* Cloud Save/Load Section */}
        <div className={`flex items-center gap-1 border-l pl-2 ${
          theme === "dark" ? "border-gray-700" : "border-gray-200"
        }`}>
          <button
            onClick={handleSaveToCloud}
            disabled={isSaving || !roomId}
            className={`p-2 rounded-lg transition-colors ${
              theme === "dark" 
                ? "hover:bg-gray-700 text-gray-300 disabled:text-gray-600" 
                : "hover:bg-gray-100 text-gray-600 disabled:text-gray-300"
            } disabled:cursor-not-allowed`}
            title="Save to cloud"
          >
            {isSaving ? <Loader2 size={18} className="animate-spin" /> : <Save size={18} />}
          </button>
          <button
            onClick={handleLoadFromCloud}
            disabled={isLoading || !roomId}
            className={`p-2 rounded-lg transition-colors ${
              theme === "dark" 
                ? "hover:bg-gray-700 text-gray-300 disabled:text-gray-600" 
                : "hover:bg-gray-100 text-gray-600 disabled:text-gray-300"
            } disabled:cursor-not-allowed`}
            title="Load from cloud"
          >
            {isLoading ? <Loader2 size={18} className="animate-spin" /> : <FolderOpen size={18} />}
          </button>
          {saveStatus && (
            <span className={`text-xs px-2 py-1 rounded ${
              saveStatus.includes("failed") 
                ? "text-red-500" 
                : theme === "dark" ? "text-green-400" : "text-green-600"
            }`}>
              {saveStatus}
            </span>
          )}
        </div>

        {/* Theme Toggle */}
        <button
          onClick={toggleTheme}
          className={`p-2 rounded-lg transition-colors ${
            theme === "dark" ? "hover:bg-gray-700 text-yellow-400" : "hover:bg-gray-100 text-gray-600"
          }`}
          title={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
        >
          {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
        </button>
      </div>
    </div>
  );
}

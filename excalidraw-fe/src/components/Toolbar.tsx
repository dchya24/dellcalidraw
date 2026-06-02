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
  Files,
} from "lucide-react";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useThemeStore } from "../store/useThemeStore";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import CollaborationPanel from "./CollaborationPanel";
import { apiService } from "../services/api";
import {
  exportActiveSheetJSON,
  exportFileAllSheets,
  exportActiveSheetPNG,
  exportActiveSheetSVG,
  handleFileImport,
  saveActiveSheetToCloud,
  loadActiveSheetFromCloud,
} from "../services/exportImportService";

interface ToolbarProps {
  excalidrawAPI: ExcalidrawImperativeAPI | null;
  onToggleSidebar?: () => void;
  username?: string;
  isAuthenticated?: boolean;
  onOpenRoomSettings?: () => void;
  onSaveToCloud?: () => void;
}

export default function Toolbar({ excalidrawAPI, onToggleSidebar, username = "Guest", isAuthenticated = false, onOpenRoomSettings, onSaveToCloud }: ToolbarProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { theme, toggleTheme } = useThemeStore();
  const {
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

  // Save canvas to cloud
  const handleSaveToCloud = async () => {
    if (isSaving) return;
    setIsSaving(true);
    setSaveStatus(null);

    const result = await saveActiveSheetToCloud(apiService);
    if (result.success) {
      setSaveStatus(`Saved ${result.count} elements`);
    } else {
      setSaveStatus(result.error || "Save failed");
    }
    setIsSaving(false);
  };

  // Load canvas from cloud
  const handleLoadFromCloud = async () => {
    if (!excalidrawAPI || isLoading) return;
    setIsLoading(true);
    setSaveStatus(null);

    const result = await loadActiveSheetFromCloud(excalidrawAPI, apiService);
    if (result.success) {
      setSaveStatus(`Loaded ${result.count} elements`);
    } else {
      setSaveStatus(result.error || "Load failed");
    }
    setIsLoading(false);
  };

  // Export handlers
  const handleExportJSON = () => {
    if (!excalidrawAPI) return;
    exportActiveSheetJSON(excalidrawAPI);
  };

  const handleExportAllSheets = () => {
    if (!excalidrawAPI) return;
    exportFileAllSheets(excalidrawAPI);
  };

  const handleExportPNG = async () => {
    if (!excalidrawAPI) return;
    await exportActiveSheetPNG(excalidrawAPI);
  };

  const handleExportSVG = async () => {
    if (!excalidrawAPI) return;
    await exportActiveSheetSVG(excalidrawAPI);
  };

  // Import handlers
  const handleImport = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      const content = event.target?.result as string;
      if (!excalidrawAPI) return;

      const success = handleFileImport(content, excalidrawAPI);
      if (!success) {
        alert("Unrecognized file format");
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
                <span>Export Sheet (.excalidraw)</span>
              </button>
              <button
                onClick={handleExportAllSheets}
                className={`flex items-center gap-3 px-4 py-2.5 w-full text-left text-sm border-b ${
                  theme === "dark" ? "hover:bg-gray-700 text-gray-200 border-gray-700" : "hover:bg-gray-100 text-gray-700 border-gray-100"
                }`}
              >
                <Files size={16} />
                <span>Export File (all sheets)</span>
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
            onClick={onSaveToCloud || handleSaveToCloud}
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

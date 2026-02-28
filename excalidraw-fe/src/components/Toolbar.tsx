import { useRef, useEffect } from "react";
import {
  Download,
  Upload,
  FileJson,
  Image,
  FileType,
  Moon,
  Sun,
  Sidebar,
} from "lucide-react";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useThemeStore } from "../store/useThemeStore";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import { exportToSvg, exportToBlob } from "@excalidraw/excalidraw";
import CollaborationPanel from "./CollaborationPanel";
import type { WhiteboardTab } from "../store/useWhiteboardStore";

interface ToolbarProps {
  excalidrawAPI: ExcalidrawImperativeAPI | null;
  onToggleSidebar?: () => void;
  username?: string;
}

export default function Toolbar({ excalidrawAPI, onToggleSidebar, username = "Guest" }: ToolbarProps) {
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
          onRegenerateRoomId={() => regenerateRoomId(roomId)}
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
            <div className={`absolute right-0 top-full mt-2 rounded-lg shadow-xl border opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all min-w-45 overflow-hidden ${
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

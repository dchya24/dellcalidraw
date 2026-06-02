import { useWhiteboardStore } from "../store/useWhiteboardStore";
import type { WhiteboardFile } from "../store/useWhiteboardStore";
import { useAuthStore } from "../store/useAuthStore";
import { useThemeStore } from "../store/useThemeStore";
import {
  FileText,
  Clock,
  Plus,
  FolderOpen,
  Trash2,
  Edit2,
  Cloud,
  CloudOff,
  Search,
  Copy,
  Download,
  ArrowUpDown,
  X,
} from "lucide-react";
import { useState, useEffect, useRef, useMemo } from "react";
import { exportFileAllSheets } from "../services/exportImportService";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";

type SortOption = "name-asc" | "name-desc" | "date-desc" | "date-asc" | "sheets-desc";

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
  excalidrawAPI?: ExcalidrawImperativeAPI | null;
}

export default function Sidebar({ isOpen, onClose, excalidrawAPI }: SidebarProps) {
  const {
    files,
    activeFileId,
    createFile,
    deleteFile,
    renameFile,
    duplicateFile,
    setActiveFile,
    isLoading,
    syncStatus,
  } = useWhiteboardStore();

  const { isAuthenticated } = useAuthStore();
  const { theme } = useThemeStore();

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState<SortOption>("date-desc");
  const [showSortMenu, setShowSortMenu] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const sortMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (editingId && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editingId]);

  // Close sort menu on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (sortMenuRef.current && !sortMenuRef.current.contains(e.target as Node)) {
        setShowSortMenu(false);
      }
    };
    if (showSortMenu) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [showSortMenu]);

  // Filter and sort files
  const filteredFiles = useMemo(() => {
    let result = [...files];

    // Filter by search
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter((f) => f.name.toLowerCase().includes(query));
    }

    // Sort
    switch (sortBy) {
      case "name-asc":
        result.sort((a, b) => a.name.localeCompare(b.name));
        break;
      case "name-desc":
        result.sort((a, b) => b.name.localeCompare(a.name));
        break;
      case "date-desc":
        result.sort((a, b) => b.lastModified - a.lastModified);
        break;
      case "date-asc":
        result.sort((a, b) => a.lastModified - b.lastModified);
        break;
      case "sheets-desc":
        result.sort((a, b) => b.tabs.length - a.tabs.length);
        break;
    }

    return result;
  }, [files, searchQuery, sortBy]);

  if (!isOpen) return null;

  const formatDate = (timestamp: number) => {
    const now = Date.now();
    const diff = now - timestamp;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return "Just now";
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;
    return new Date(timestamp).toLocaleDateString();
  };

  const handleStartEdit = (fileId: string, currentName: string) => {
    setEditingId(fileId);
    setEditValue(currentName);
  };

  const handleRenameSubmit = (fileId: string) => {
    if (editValue.trim()) {
      renameFile(fileId, editValue.trim());
    }
    setEditingId(null);
  };

  const handleFileClick = (fileId: string) => {
    setActiveFile(fileId);
  };

  const handleCreateFile = () => {
    createFile();
  };

  const handleDuplicate = (e: React.MouseEvent, fileId: string) => {
    e.stopPropagation();
    duplicateFile(fileId);
  };

  const handleExportFile = (e: React.MouseEvent, _file: WhiteboardFile) => {
    e.stopPropagation();
    if (!excalidrawAPI) return;

    // Set active file first, then export
    setActiveFile(_file.id);
    // Small delay to let state update
    setTimeout(() => {
      exportFileAllSheets(excalidrawAPI);
    }, 100);
  };

  const sortLabels: Record<SortOption, string> = {
    "name-asc": "Name (A-Z)",
    "name-desc": "Name (Z-A)",
    "date-desc": "Newest first",
    "date-asc": "Oldest first",
    "sheets-desc": "Most sheets",
  };

  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/20 z-40" onClick={onClose} />

      {/* Sidebar */}
      <div
        className={`fixed left-0 top-0 bottom-10 w-72 z-50 shadow-lg overflow-y-auto transition-transform ${
          theme === "dark"
            ? "bg-gray-800 border-r border-gray-700"
            : "bg-white border-r border-gray-200"
        }`}
      >
        {/* Header */}
        <div
          className={`p-4 border-b ${
            theme === "dark" ? "border-gray-700" : "border-gray-200"
          }`}
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <h2
                className={`text-lg font-semibold ${
                  theme === "dark" ? "text-white" : "text-gray-900"
                }`}
              >
                Files
              </h2>
              {isAuthenticated && (
                <span className="flex items-center gap-1">
                  {syncStatus === "synced" ? (
                    <Cloud size={14} className="text-green-500" />
                  ) : syncStatus === "error" ? (
                    <CloudOff size={14} className="text-red-500" />
                  ) : (
                    <Cloud size={14} className="text-blue-500 animate-pulse" />
                  )}
                </span>
              )}
            </div>
            <button
              onClick={handleCreateFile}
              disabled={isLoading}
              className={`p-2 rounded-lg transition-colors disabled:opacity-50 ${
                theme === "dark"
                  ? "bg-gray-700 hover:bg-gray-600 text-white"
                  : "bg-gray-100 hover:bg-gray-200 text-gray-700"
              }`}
              title="New file"
            >
              <Plus size={18} />
            </button>
          </div>

          {/* Search & Sort Row */}
          <div className="flex items-center gap-2">
            {/* Search */}
            <div
              className={`flex-1 flex items-center gap-2 px-2.5 py-1.5 rounded-lg border ${
                theme === "dark"
                  ? "bg-gray-700/50 border-gray-600"
                  : "bg-gray-50 border-gray-200"
              }`}
            >
              <Search
                size={14}
                className={theme === "dark" ? "text-gray-400" : "text-gray-400"}
              />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search files..."
                className={`flex-1 bg-transparent outline-none text-xs ${
                  theme === "dark"
                    ? "text-white placeholder-gray-500"
                    : "text-gray-900 placeholder-gray-400"
                }`}
              />
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery("")}
                  className={`p-0.5 rounded ${
                    theme === "dark"
                      ? "hover:bg-gray-600 text-gray-400"
                      : "hover:bg-gray-200 text-gray-400"
                  }`}
                >
                  <X size={12} />
                </button>
              )}
            </div>

            {/* Sort */}
            <div className="relative" ref={sortMenuRef}>
              <button
                onClick={() => setShowSortMenu(!showSortMenu)}
                className={`p-1.5 rounded-lg transition-colors ${
                  theme === "dark"
                    ? "hover:bg-gray-700 text-gray-400"
                    : "hover:bg-gray-100 text-gray-500"
                }`}
                title={`Sort: ${sortLabels[sortBy]}`}
              >
                <ArrowUpDown size={14} />
              </button>
              {showSortMenu && (
                <div
                  className={`absolute right-0 top-full mt-1 rounded-lg shadow-xl border z-10 min-w-36 overflow-hidden ${
                    theme === "dark"
                      ? "bg-gray-800 border-gray-700"
                      : "bg-white border-gray-200"
                  }`}
                >
                  {(Object.keys(sortLabels) as SortOption[]).map((option) => (
                    <button
                      key={option}
                      onClick={() => {
                        setSortBy(option);
                        setShowSortMenu(false);
                      }}
                      className={`w-full text-left px-3 py-2 text-xs transition-colors ${
                        sortBy === option
                          ? theme === "dark"
                            ? "bg-gray-700 text-blue-400"
                            : "bg-blue-50 text-blue-600"
                          : theme === "dark"
                            ? "hover:bg-gray-700 text-gray-300"
                            : "hover:bg-gray-50 text-gray-700"
                      }`}
                    >
                      {sortLabels[option]}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* File count */}
          <p
            className={`text-xs mt-2 ${
              theme === "dark" ? "text-gray-500" : "text-gray-400"
            }`}
          >
            {filteredFiles.length}
            {searchQuery ? ` of ${files.length}` : ""}{" "}
            {filteredFiles.length === 1 ? "file" : "files"}
            {isAuthenticated ? " · cloud" : " · local"}
          </p>
        </div>

        {/* Files List */}
        <div className="p-2">
          {filteredFiles.map((file) => (
            <div
              key={file.id}
              onClick={() => handleFileClick(file.id)}
              className={`p-3 rounded-lg mb-2 transition-colors cursor-pointer ${
                activeFileId === file.id
                  ? theme === "dark"
                    ? "bg-gray-700 border border-gray-600"
                    : "bg-blue-50 border border-blue-200"
                  : theme === "dark"
                    ? "bg-gray-700/30 hover:bg-gray-700/50"
                    : "bg-gray-50 hover:bg-gray-100"
              }`}
            >
              {/* File Header */}
              <div className="flex items-center gap-2 mb-2">
                <FolderOpen
                  size={16}
                  className={
                    activeFileId === file.id
                      ? "text-blue-500"
                      : theme === "dark"
                        ? "text-gray-400"
                        : "text-gray-500"
                  }
                />

                {editingId === file.id ? (
                  <input
                    ref={inputRef}
                    type="text"
                    value={editValue}
                    onChange={(e) => setEditValue(e.target.value)}
                    onBlur={() => handleRenameSubmit(file.id)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") handleRenameSubmit(file.id);
                      if (e.key === "Escape") setEditingId(null);
                    }}
                    onClick={(e) => e.stopPropagation()}
                    className={`flex-1 bg-transparent outline-none text-sm font-medium ${
                      theme === "dark" ? "text-white" : "text-gray-900"
                    }`}
                  />
                ) : (
                  <span
                    className={`font-medium text-sm truncate flex-1 ${
                      theme === "dark" ? "text-white" : "text-gray-900"
                    }`}
                  >
                    {file.name}
                    {file.isCloud && (
                      <Cloud
                        size={12}
                        className="inline ml-1 text-green-500"
                      />
                    )}
                  </span>
                )}

                {editingId !== file.id && (
                  <div className="flex gap-0.5">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleStartEdit(file.id, file.name);
                      }}
                      className={`p-1 rounded transition-colors ${
                        theme === "dark"
                          ? "hover:bg-gray-600 text-gray-400 hover:text-white"
                          : "hover:bg-gray-200 text-gray-400 hover:text-gray-600"
                      }`}
                      title="Rename"
                    >
                      <Edit2 size={12} />
                    </button>
                    <button
                      onClick={(e) => handleDuplicate(e, file.id)}
                      className={`p-1 rounded transition-colors ${
                        theme === "dark"
                          ? "hover:bg-gray-600 text-gray-400 hover:text-white"
                          : "hover:bg-gray-200 text-gray-400 hover:text-gray-600"
                      }`}
                      title="Duplicate"
                    >
                      <Copy size={12} />
                    </button>
                    <button
                      onClick={(e) => handleExportFile(e, file)}
                      className={`p-1 rounded transition-colors ${
                        theme === "dark"
                          ? "hover:bg-gray-600 text-gray-400 hover:text-white"
                          : "hover:bg-gray-200 text-gray-400 hover:text-gray-600"
                      }`}
                      title="Export file"
                    >
                      <Download size={12} />
                    </button>
                    {files.length > 1 && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          deleteFile(file.id);
                        }}
                        className={`p-1 rounded transition-colors ${
                          theme === "dark"
                            ? "hover:bg-red-900/30 text-gray-400 hover:text-red-400"
                            : "hover:bg-red-100 text-gray-400 hover:text-red-600"
                        }`}
                        title="Delete"
                      >
                        <Trash2 size={12} />
                      </button>
                    )}
                  </div>
                )}
              </div>

              {/* File Info */}
              <div className="space-y-1.5 ml-6">
                {/* Sheet Count */}
                <div className="flex items-center gap-2">
                  <FileText
                    size={12}
                    className={
                      theme === "dark" ? "text-gray-500" : "text-gray-400"
                    }
                  />
                  <span
                    className={`text-xs ${
                      theme === "dark" ? "text-gray-400" : "text-gray-500"
                    }`}
                  >
                    {file.tabs.length}{" "}
                    {file.tabs.length === 1 ? "sheet" : "sheets"}
                  </span>
                </div>

                {/* Last Modified */}
                <div className="flex items-center gap-2">
                  <Clock
                    size={12}
                    className={
                      theme === "dark" ? "text-gray-500" : "text-gray-400"
                    }
                  />
                  <span
                    className={`text-xs ${
                      theme === "dark" ? "text-gray-400" : "text-gray-500"
                    }`}
                  >
                    {formatDate(file.lastModified)}
                  </span>
                </div>
              </div>
            </div>
          ))}

          {/* No results */}
          {filteredFiles.length === 0 && searchQuery && (
            <div className="text-center py-6">
              <Search
                size={24}
                className={
                  theme === "dark" ? "text-gray-600 mx-auto" : "text-gray-300 mx-auto"
                }
              />
              <p
                className={`mt-2 text-sm ${
                  theme === "dark" ? "text-gray-500" : "text-gray-400"
                }`}
              >
                No files matching "{searchQuery}"
              </p>
            </div>
          )}

          {/* Empty state */}
          {files.length === 0 && !isLoading && (
            <div className="text-center py-8">
              <FolderOpen
                size={32}
                className={
                  theme === "dark" ? "text-gray-600 mx-auto" : "text-gray-300 mx-auto"
                }
              />
              <p
                className={`mt-2 text-sm ${
                  theme === "dark" ? "text-gray-500" : "text-gray-400"
                }`}
              >
                No files yet
              </p>
              <button
                onClick={handleCreateFile}
                className={`mt-2 px-4 py-2 rounded-lg text-sm ${
                  theme === "dark"
                    ? "bg-blue-600 hover:bg-blue-500 text-white"
                    : "bg-blue-500 hover:bg-blue-600 text-white"
                }`}
              >
                Create your first file
              </button>
            </div>
          )}
        </div>
      </div>
    </>
  );
}

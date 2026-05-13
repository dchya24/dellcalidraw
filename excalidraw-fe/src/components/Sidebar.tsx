import { useFileStore } from "../store/useFileStore";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useAuthStore } from "../store/useAuthStore";
import { useThemeStore } from "../store/useThemeStore";
import { FileText, Clock, Plus, FolderOpen, Trash2, Edit2, Cloud, CloudOff } from "lucide-react";
import { useState, useEffect, useRef } from "react";

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function Sidebar({ isOpen, onClose }: SidebarProps) {
  const { 
    localFiles, 
    activeFileId, 
    createFile, 
    deleteFile, 
    renameFile, 
    setActiveFile,
    isLoading,
    syncStatus
  } = useFileStore();
  
  // Whiteboard store for tabs
  const { 
    files: whiteboardFiles, 
    setActiveFile: setWhiteboardActiveFile 
  } = useWhiteboardStore();
  
  const { isAuthenticated } = useAuthStore();
  const { theme } = useThemeStore();
  
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editingId && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editingId]);

  if (!isOpen) return null;

  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleString();
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
    setWhiteboardActiveFile(fileId);
  };

  const handleCreateFile = () => {
    createFile();
  };

  // Get tab count for a file (from whiteboard store if available)
  const getTabCount = (fileId: string): number => {
    const wbFile = whiteboardFiles.find(f => f.id === fileId);
    return wbFile?.tabs.length || 1;
  };

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/20 z-40"
        onClick={onClose}
      />

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
          className={`p-4 border-b flex items-center justify-between ${
            theme === "dark" ? "border-gray-700" : "border-gray-200"
          }`}
        >
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <h2
                className={`text-lg font-semibold ${
                  theme === "dark" ? "text-white" : "text-gray-900"
                }`}
              >
                Files
              </h2>
              {/* Cloud status indicator */}
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
            <p
              className={`text-xs mt-1 ${
                theme === "dark" ? "text-gray-400" : "text-gray-500"
              }`}
            >
              {localFiles.length} {localFiles.length === 1 ? "file" : "files"}
              {isAuthenticated ? " (cloud synced)" : " (local only)"}
            </p>
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

        {/* Files List */}
        <div className="p-2">
          {localFiles.map((file) => (
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
                      <Cloud size={12} className="inline ml-1 text-green-500" />
                    )}
                  </span>
                )}

                {editingId !== file.id && (
                  <div className="flex gap-1">
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
                    >
                      <Edit2 size={12} />
                    </button>
                    {localFiles.length > 1 && (
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
                    {getTabCount(file.id)} {getTabCount(file.id) === 1 ? "sheet" : "sheets"}
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
          
          {localFiles.length === 0 && !isLoading && (
            <div className="text-center py-8">
              <FolderOpen size={32} className={theme === "dark" ? "text-gray-600" : "text-gray-300"} />
              <p className={`mt-2 text-sm ${theme === "dark" ? "text-gray-500" : "text-gray-400"}`}>
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
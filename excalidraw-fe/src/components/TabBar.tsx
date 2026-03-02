import { useState, useRef, useEffect } from "react";
import { Plus, X, List } from "lucide-react";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useThemeStore } from "../store/useThemeStore";

interface TabBarProps {
  onTabChange: (tabId: string) => void;
  onAddTab: () => void;
  onDeleteRequest?: (tabId: string) => void;
  handleFloatingTabOpen: () => void;
}

export default function TabBar({ onTabChange, onAddTab, onDeleteRequest, handleFloatingTabOpen }: TabBarProps) {
  const { getActiveFile, removeTab, renameTab, setActiveTab } = useWhiteboardStore();
  const { theme } = useThemeStore();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const [showDropdown, setShowDropdown] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const activeFile = getActiveFile();
  const tabs = activeFile?.tabs || [];
  const activeTabId = activeFile?.activeTabId || "";

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowDropdown(false);
      }
    };

    if (showDropdown) {
      document.addEventListener("mousedown", handleClickOutside);
    }

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [showDropdown]);

  const handleTabClick = (tabId: string) => {
    if (tabId !== activeTabId) {
      onTabChange(tabId);
      setActiveTab(tabId);
    }
  };

  const handleDoubleClick = (tabId: string, currentTitle: string) => {
    setEditingId(tabId);
    setEditValue(currentTitle);
  };

  const handleRenameSubmit = (tabId: string) => {
    if (editValue.trim()) {
      renameTab(tabId, editValue.trim());
    }
    setEditingId(null);
  };

  const handleAddTab = () => {
    onAddTab();
  };

  const handleToggleDropdown = () => {
    setShowDropdown(!showDropdown);
    handleFloatingTabOpen();
  };

  const handleRemoveTab = (e: React.MouseEvent, tabId: string) => {
    e.stopPropagation();
    if (tabs.length > 1) {
      if (onDeleteRequest) {
        onDeleteRequest(tabId);
      } else {
        removeTab(tabId);
      }
    }
  };

  return (
    <div
      className={`flex items-center h-12 pr-2 gap-1 pb-1 overflow-x-auto relative z-50 pl-3 ${
        theme === "dark"
          ? "bg-gray-800 border-t border-gray-700"
          : "bg-gray-100 border-t border-gray-300"
      }`}
    >
      {/* Tab List Dropdown */}
      <div className="relative" ref={dropdownRef}>
        <button
          ref={buttonRef}
          onClick={handleToggleDropdown}
          className={`p-1.5 rounded transition-colors ${
            theme === "dark"
              ? "hover:bg-gray-700 text-gray-300"
              : "hover:bg-gray-300"
          }`}
          title="Tab list"
        >
          <List size={18} />
        </button>
      </div>

      {tabs.map((tab) => (
        <div
          key={tab.id}
          onClick={() => handleTabClick(tab.id)}
          className={`flex items-center gap-1 px-3 py-1 rounded-md cursor-pointer min-w-[25] max-w-[45] group transition-colors ${
            activeTabId === tab.id
              ? theme === "dark"
                ? "bg-gray-700 border-t border-l border-r border-gray-600 -mb-px text-gray-300"
                : "bg-white border-t border-l border-r border-gray-300 -mb-px"
              : theme === "dark"
                ? "bg-gray-700/50 hover:bg-gray-600 text-gray-300"
                : "bg-gray-200 hover:bg-gray-300"
          }`}
        >
          {editingId === tab.id ? (
            <input
              type="text"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onBlur={() => handleRenameSubmit(tab.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleRenameSubmit(tab.id);
                if (e.key === "Escape") setEditingId(null);
              }}
              className="w-full bg-transparent outline-none text-sm"
              autoFocus
            />
          ) : (
            <span
              onDoubleClick={() => handleDoubleClick(tab.id, tab.title)}
              className="text-sm truncate flex-1"
            >
              {tab.title}
            </span>
          )}
          {tabs.length > 1 && (
            <button
              onClick={(e) => handleRemoveTab(e, tab.id)}
              className={`opacity-0 group-hover:opacity-100 rounded p-0.5 transition-opacity ${
                theme === "dark" ? "hover:bg-gray-600" : "hover:bg-gray-400"
              }`}
            >
              <X size={14} />
            </button>
          )}
        </div>
      ))}
      <button
        onClick={handleAddTab}
        className={`p-1.5 rounded transition-colors ${
          theme === "dark" ? "hover:bg-gray-700 text-gray-300" : "hover:bg-gray-300"
        }`}
        title="Add new tab"
      >
        <Plus size={18} />
      </button>
    </div>
  );
}

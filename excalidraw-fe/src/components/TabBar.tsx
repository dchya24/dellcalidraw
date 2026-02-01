import { useState, useRef, useEffect } from "react";
import { createPortal } from "react-dom";
import { Plus, X, List } from "lucide-react";
import { useWhiteboardStore } from "../store/useWhiteboardStore";
import { useThemeStore } from "../store/useThemeStore";

interface TabBarProps {
  onTabChange: (tabId: string) => void;
  onAddTab: () => void;
  onDeleteRequest?: (tabId: string) => void;
}

export default function TabBar({ onTabChange, onAddTab, onDeleteRequest }: TabBarProps) {
  const { getActiveFile, removeTab, renameTab, setActiveTab } = useWhiteboardStore();
  const { theme } = useThemeStore();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");
  const [showDropdown, setShowDropdown] = useState(false);
  const [dropdownPosition, setDropdownPosition] = useState({ top: 0, left: 0 });
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
    if (!showDropdown && buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      setDropdownPosition({
        top: rect.bottom + 8,
        left: rect.left
      });
    }
    setShowDropdown(!showDropdown);
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

        {showDropdown && createPortal(
          <div
            className={`fixed w-64 rounded-lg shadow-xl border max-h-96 overflow-y-auto ${
              theme === "dark"
                ? "bg-gray-800 border-gray-700"
                : "bg-white border-gray-200"
            }`}
            style={{
              top: `${dropdownPosition.top}px`,
              left: `${dropdownPosition.left}px`,
              zIndex: 9999
            }}
            ref={dropdownRef}
          >
            <div className="p-2 z-50">
              {tabs.map((tab) => (
                <div
                  key={tab.id}
                  onClick={() => {
                    handleTabClick(tab.id);
                    setShowDropdown(false);
                  }}
                  className={`flex items-center justify-between p-2 rounded cursor-pointer transition-colors ${
                    activeTabId === tab.id
                      ? theme === "dark"
                        ? "bg-gray-700"
                        : "bg-blue-50"
                      : theme === "dark"
                        ? "hover:bg-gray-700/50"
                        : "hover:bg-gray-100"
                  }`}
                >
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <span
                      className={`text-sm truncate ${
                        activeTabId === tab.id
                          ? theme === "dark"
                            ? "text-white font-medium"
                            : "text-gray-900 font-medium"
                          : theme === "dark"
                            ? "text-gray-300"
                            : "text-gray-700"
                      }`}
                    >
                      {tab.title}
                    </span>
                  </div>
                  {tabs.length > 1 && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleRemoveTab(e, tab.id);
                        setShowDropdown(false);
                      }}
                      className={`p-1 rounded transition-colors ${
                        theme === "dark"
                          ? "hover:bg-red-900/30 text-gray-400 hover:text-red-400"
                          : "hover:bg-red-100 text-gray-400 hover:text-red-600"
                      }`}
                      title="Delete tab"
                    >
                      <X size={14} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>,
          document.body
        )}
      </div>

      {tabs.map((tab) => (
        <div
          key={tab.id}
          onClick={() => handleTabClick(tab.id)}
          className={`flex items-center gap-1 px-3 py-1 rounded-md cursor-pointer min-w-[25] max-w-[45] group transition-colors ${
            activeTabId === tab.id
              ? theme === "dark"
                ? "bg-gray-700 border-t border-l border-r border-gray-600 -mb-px"
                : "bg-white border-t border-l border-r border-gray-300 -mb-px"
              : theme === "dark"
                ? "bg-gray-700/50 hover:bg-gray-600"
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

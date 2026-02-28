import { X } from "lucide-react";
import { useThemeStore } from "../store/useThemeStore";
import { useWhiteboardStore, WhiteboardTab } from "../store/useWhiteboardStore";

interface FloatingTabProps {
  tabs: WhiteboardTab[];
  activeTabId: string;
  onTabChange: (tabId: string) => void;
  onDeleteRequest: (tabId: string) => void;
}

export default function FloatingTab({ tabs, activeTabId, onTabChange, onDeleteRequest }: FloatingTabProps) {
  const { setActiveTab } = useWhiteboardStore();
  const { theme } = useThemeStore();

  const handleTabClick = (tabId: string) => {
    if (tabId !== activeTabId) {
      onTabChange(tabId);
      setActiveTab(tabId);
    }
  };

  return (
    <div
      className={`absolute bottom-2 left-1 z-10 w-48 rounded-lg shadow-xl border max-h-96 overflow-y-auto ${
        theme === "dark"
          ? "bg-gray-800 border-gray-700"
          : "bg-white border-gray-200"
      }`}
    >
      <div className="p-2">
        {tabs.map((tab) => (
          <div
            key={tab.id}
            onClick={(e) => {
              console.log("Tab clicked:", tab.id);
              e.stopPropagation();
              handleTabClick(tab.id);
            }}
            className={`flex items-center justify-between p-1 rounded cursor-pointer transition-colors ${
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
                  onDeleteRequest(tab.id);
                }}
                className={`p-1 rounded transition-colors ${
                  theme === "dark"
                    ? "hover:bg-red-900/30 text-gray-400 hover:text-red-400"
                    : "hover:bg-red-100 text-gray-400 hover:text-red-600"
                }`}
                title="Delete tab"
              >
                <X size={12} />
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

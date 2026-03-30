import { useState } from "react";
import { useThemeStore } from "../store/useThemeStore";

interface ConflictEntry {
  id: string;
  userId: string;
  username: string;
  color: string;
  timestamp: number;
  description: string;
  elementCount: number;
}

interface ConflictResolutionPanelProps {
  conflicts: ConflictEntry[];
  onDismiss?: (id: string) => void;
  onDismissAll?: () => void;
}

export default function ConflictResolutionPanel({
  conflicts,
  onDismiss,
  onDismissAll,
}: ConflictResolutionPanelProps) {
  const { theme } = useThemeStore();
  const [isExpanded, setIsExpanded] = useState(true);

  if (conflicts.length === 0) return null;

  const formatTime = (timestamp: number) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };

  return (
    <div
      className={`absolute bottom-20 right-4 z-50 w-80 rounded-lg shadow-xl overflow-hidden transition-all ${
        theme === "dark" ? "bg-gray-800 border border-gray-700" : "bg-white border border-gray-200"
      }`}
    >
      {/* Header */}
      <div
        className={`flex items-center justify-between px-4 py-3 border-b ${
          theme === "dark" ? "border-gray-700 bg-gray-700/50" : "border-gray-200 bg-gray-50"
        }`}
      >
        <div className="flex items-center gap-2">
          <svg
            className="w-5 h-5 text-yellow-500"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <span className={`font-medium ${theme === "dark" ? "text-white" : "text-gray-900"}`}>
            Changes from Others
          </span>
          <span
            className={`px-2 py-0.5 rounded-full text-xs ${
              theme === "dark" ? "bg-gray-700 text-gray-300" : "bg-gray-200 text-gray-700"
            }`}
          >
            {conflicts.length}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className={`p-1 rounded transition-colors ${
              theme === "dark" ? "hover:bg-gray-700" : "hover:bg-gray-200"
            }`}
          >
            <svg
              className={`w-4 h-4 ${theme === "dark" ? "text-gray-400" : "text-gray-600"} transition-transform ${
                isExpanded ? "rotate-180" : ""
              }`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          <button
            onClick={onDismissAll}
            className={`p-1 rounded transition-colors ${
              theme === "dark" ? "hover:bg-gray-700" : "hover:bg-gray-200"
            }`}
            title="Dismiss all"
          >
            <svg
              className={`w-4 h-4 ${theme === "dark" ? "text-gray-400" : "text-gray-600"}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      {/* Conflict List */}
      {isExpanded && (
        <div className="max-h-64 overflow-y-auto">
          {conflicts.map((conflict) => (
            <div
              key={conflict.id}
              className={`flex items-start gap-3 px-4 py-3 border-b last:border-b-0 ${
                theme === "dark"
                  ? "border-gray-700 hover:bg-gray-700/30"
                  : "border-gray-100 hover:bg-gray-50"
              } transition-colors`}
            >
              {/* User Avatar */}
              <div
                className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium flex-shrink-0"
                style={{ backgroundColor: conflict.color, color: "#fff" }}
              >
                {conflict.username.charAt(0).toUpperCase()}
              </div>

              {/* Content */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2">
                  <p
                    className={`font-medium text-sm truncate ${
                      theme === "dark" ? "text-white" : "text-gray-900"
                    }`}
                  >
                    {conflict.username}
                  </p>
                  <span
                    className={`text-xs flex-shrink-0 ${
                      theme === "dark" ? "text-gray-500" : "text-gray-400"
                    }`}
                  >
                    {formatTime(conflict.timestamp)}
                  </span>
                </div>
                <p
                  className={`text-sm mt-0.5 ${
                    theme === "dark" ? "text-gray-400" : "text-gray-600"
                  }`}
                >
                  {conflict.description}
                </p>
                <p
                  className={`text-xs mt-1 ${
                    theme === "dark" ? "text-gray-500" : "text-gray-500"
                  }`}
                >
                  {conflict.elementCount} element{conflict.elementCount !== 1 ? "s" : ""} modified
                </p>
              </div>

              {/* Dismiss Button */}
              <button
                onClick={() => onDismiss?.(conflict.id)}
                className={`p-1 rounded flex-shrink-0 transition-colors ${
                  theme === "dark" ? "hover:bg-gray-700" : "hover:bg-gray-200"
                }`}
              >
                <svg
                  className={`w-4 h-4 ${theme === "dark" ? "text-gray-500" : "text-gray-400"}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Footer */}
      {isExpanded && conflicts.length > 0 && (
        <div
          className={`px-4 py-2 text-xs border-t ${
            theme === "dark"
              ? "border-gray-700 text-gray-500 bg-gray-700/30"
              : "border-gray-200 text-gray-500 bg-gray-50"
          }`}
        >
          Changes are automatically synchronized. Conflicts are resolved using last-write-wins.
        </div>
      )}
    </div>
  );
}

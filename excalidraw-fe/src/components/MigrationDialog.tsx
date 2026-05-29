import { CloudUpload } from "lucide-react";
import { useThemeStore } from "../store/useThemeStore";
import type { WhiteboardFile } from "../store/useWhiteboardStore";

interface MigrationDialogProps {
  isOpen: boolean;
  pendingFiles: WhiteboardFile[];
  onMerge: () => void;
  onDiscard: () => void;
}

export default function MigrationDialog({
  isOpen,
  pendingFiles,
  onMerge,
  onDiscard,
}: MigrationDialogProps) {
  const { theme } = useThemeStore();

  if (!isOpen || pendingFiles.length === 0) return null;
  const isDark = theme === "dark";

  return (
    <div className="fixed inset-0 flex items-center justify-center z-50">
      <div className="absolute inset-0 bg-black/50" />

      <div
        className={`relative rounded-lg shadow-xl max-w-md w-full mx-4 p-6 ${
          isDark ? "bg-gray-800 text-white" : "bg-white text-gray-900"
        }`}
      >
        <div className="flex items-start gap-4">
          <div className="shrink-0">
            <CloudUpload
              className={`w-6 h-6 ${
                isDark ? "text-blue-400" : "text-blue-500"
              }`}
            />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold mb-2">
              Keep your guest drawings?
            </h3>
            <p
              className={`text-sm ${
                isDark ? "text-gray-300" : "text-gray-600"
              }`}
            >
              You have {pendingFiles.length} file
              {pendingFiles.length === 1 ? "" : "s"} created while signed out.
              Upload them to your account, or discard and use your cloud files
              only.
            </p>

            <ul
              className={`mt-3 space-y-1 text-xs max-h-32 overflow-y-auto ${
                isDark ? "text-gray-400" : "text-gray-500"
              }`}
            >
              {pendingFiles.slice(0, 6).map((f) => (
                <li key={f.id} className="truncate">
                  • {f.name} ({f.tabs.length} sheet
                  {f.tabs.length === 1 ? "" : "s"})
                </li>
              ))}
              {pendingFiles.length > 6 && (
                <li>…and {pendingFiles.length - 6} more</li>
              )}
            </ul>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-6">
          <button
            onClick={onDiscard}
            className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
              isDark
                ? "bg-gray-700 hover:bg-gray-600 text-white"
                : "bg-gray-100 hover:bg-gray-200 text-gray-700"
            }`}
          >
            Discard
          </button>
          <button
            onClick={onMerge}
            className="px-4 py-2 rounded-md text-sm font-medium bg-blue-600 hover:bg-blue-700 text-white transition-colors"
          >
            Upload to account
          </button>
        </div>
      </div>
    </div>
  );
}

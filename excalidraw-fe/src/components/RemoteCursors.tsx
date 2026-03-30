import { useEffect, useState, useCallback } from "react";
import { cursorService } from "../services/cursorService";
import type { RemoteCursor } from "../types/websocket";
import { useThemeStore } from "../store/useThemeStore";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";

interface RemoteCursorWithPosition extends RemoteCursor {
  x: number;
  y: number;
}

interface RemoteCursorsProps {
  excalidrawAPI: ExcalidrawImperativeAPI | null;
}

export default function RemoteCursors({ excalidrawAPI }: RemoteCursorsProps) {
  const { theme } = useThemeStore();
  const [cursors, setCursors] = useState<RemoteCursorWithPosition[]>([]);

  // Transform canvas coordinates to screen coordinates
  const transformToScreen = useCallback((canvasX: number, canvasY: number) => {
    if (!excalidrawAPI) return { x: 0, y: 0 };

    const appState = excalidrawAPI.getAppState();
    const zoom = appState.zoom?.value || 1;
    
    // Transform canvas coordinates to screen coordinates
    const screenX = canvasX * zoom + appState.scrollX + appState.offsetLeft;
    const screenY = canvasY * zoom + appState.scrollY + appState.offsetTop;
    
    return { x: screenX, y: screenY };
  }, [excalidrawAPI]);

  useEffect(() => {
    // Subscribe to cursor updates
    const unsubscribeUpdated = cursorService.onCursorUpdated((cursor) => {
      // Transform canvas coordinates to screen coordinates
      const { x: screenX, y: screenY } = transformToScreen(cursor.position.x, cursor.position.y);
      
      setCursors(prev => {
        const existing = prev.findIndex(c => c.userId === cursor.userId);
        const newCursor: RemoteCursorWithPosition = {
          ...cursor,
          x: screenX,
          y: screenY,
        };

        if (existing >= 0) {
          const updated = [...prev];
          updated[existing] = newCursor;
          return updated;
        } else {
          return [...prev, newCursor];
        }
      });
    });

    // Subscribe to cursor removal
    const unsubscribeRemoved = cursorService.onCursorRemoved((userId) => {
      setCursors(prev => prev.filter(c => c.userId !== userId));
    });

    return () => {
      unsubscribeUpdated();
      unsubscribeRemoved();
    };
  }, [transformToScreen]);

  // Update cursor positions when zoom/scroll changes
  useEffect(() => {
    if (!excalidrawAPI || cursors.length === 0) return;

    // Re-transform all cursors when viewport changes
    setCursors(prev => prev.map(cursor => {
      const { x: screenX, y: screenY } = transformToScreen(cursor.position.x, cursor.position.y);
      return {
        ...cursor,
        x: screenX,
        y: screenY,
      };
    }));
  }, [excalidrawAPI, transformToScreen]); // Re-run when excalidrawAPI reference changes (indicates viewport change)

  if (cursors.length === 0) return null;

  return (
    <>
      {cursors.map(cursor => (
        <div
          key={cursor.userId}
          className="pointer-events-none fixed z-50 transition-all duration-75 ease-linear"
          style={{
            left: cursor.x,
            top: cursor.y,
            transform: "translate(4px, 4px)",
          }}
        >
          {/* Cursor pointer */}
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            style={{
              filter: "drop-shadow(1px 1px 1px rgba(0,0,0,0.3))",
            }}
          >
            <path
              d="M5.5 3.21V20.8c0 .45.54.67.85.35l4.86-4.86a.5.5 0 0 1 .35-.15h6.87c.48 0 .72-.58.38-.92L5.85 2.85a.5.5 0 0 0-.35.36Z"
              fill={cursor.color}
              stroke={theme === "dark" ? "#fff" : "#000"}
              strokeWidth="1"
            />
          </svg>

          {/* Username label */}
          <div
            className="ml-5 mt-1 px-2 py-0.5 rounded text-xs font-medium whitespace-nowrap"
            style={{
              backgroundColor: cursor.color,
              color: "#fff",
              filter: "drop-shadow(1px 1px 1px rgba(0,0,0,0.3))",
            }}
          >
            {cursor.username}
          </div>
        </div>
      ))}
    </>
  );
}

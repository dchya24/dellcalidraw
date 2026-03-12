import React from 'react';
import { selectionService } from '../services/selectionService';
import type { ElementSelection } from '../types/websocket';
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types';
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types';

interface SelectionOverlayProps {
  excalidrawAPI: ExcalidrawImperativeAPI | null;
}

export default function SelectionOverlay({ excalidrawAPI }: SelectionOverlayProps) {
  const [selections, setSelections] = React.useState<ElementSelection[]>([]);

  React.useEffect(() => {
    // Subscribe to selection updates
    const unsubscribe = selectionService.onSelectionUpdated((updatedSelections) => {
      setSelections(updatedSelections);
    });

    return unsubscribe;
  }, []);

  if (!excalidrawAPI || selections.length === 0) {
    return null;
  }

  // Get current elements to calculate their positions
  const elements = excalidrawAPI.getSceneElements();

  return (
    <svg
      className="selection-overlay"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        pointerEvents: 'none',
        zIndex: 999,
      }}
    >
      {selections.map(selection => (
        <React.Fragment key={selection.userId}>
          {selection.selectedIds.map(elementId => {
            const element = elements.find((el: OrderedExcalidrawElement) => el.id === elementId);
            if (!element) return null;

            return (
              <g key={`${selection.userId}-${elementId}`}>
                {/* Selection border */}
                <rect
                  x={element.x - 2}
                  y={element.y - 2}
                  width={(element.width || 0) + 4}
                  height={(element.height || 0) + 4}
                  fill="none"
                  stroke={selection.color}
                  strokeWidth={3}
                  strokeOpacity={0.7}
                  rx={2}
                />

                {/* User label */}
                <text
                  x={element.x}
                  y={element.y - 10}
                  fill={selection.color}
                  fontSize="11"
                  fontWeight="bold"
                  opacity={0.9}
                >
                  {selection.username}
                </text>
              </g>
            );
          })}
        </React.Fragment>
      ))}
    </svg>
  );
}

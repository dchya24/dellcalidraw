import { wsService } from './websocket';
import { roomService } from './roomService';
import type {
  SelectionChangePayload,
  SelectionUpdatedPayload,
  ElementSelection
} from '../types/websocket';

class SelectionService {
  private localSelection: Set<string> = new Set();
  private remoteSelections: Map<string, ElementSelection> = new Map();
  private selectionListeners: Set<(selections: ElementSelection[]) => void> = new Set();
  private isTracking: boolean = false;
  private updateInterval: number | null = null;

  // Debounce to avoid excessive updates
  private pendingSelectionUpdate: Set<string> = new Set();
  private readonly DEBOUNCE_DELAY = 100;

  startTracking(): void {
    if (this.isTracking) {
      console.log('Selection tracking already started');
      return;
    }

    this.isTracking = true;
    this.setupMessageHandlers();

    // Start periodic updates
    this.updateInterval = setInterval(() => {
      this.flushSelectionUpdate();
    }, this.DEBOUNCE_DELAY);

    console.log('✅ Selection tracking started');
  }

  stopTracking(): void {
    if (!this.isTracking) return;

    this.isTracking = false;

    if (this.updateInterval) {
      clearInterval(this.updateInterval);
      this.updateInterval = null;
    }

    console.log('❌ Selection tracking stopped');
  }

  // Update local selection (called when user changes selection in Excalidraw)
  updateSelection(elementIds: string[]): void {
    this.localSelection = new Set(elementIds);
    this.pendingSelectionUpdate = new Set(elementIds);
  }

  private flushSelectionUpdate(): void {
    if (this.pendingSelectionUpdate.size === 0) return;

    const roomId = roomService.getCurrentRoom();
    if (!roomId || !roomService.isConnectedToRoom()) {
      this.pendingSelectionUpdate.clear();
      return;
    }

    const payload: SelectionChangePayload = {
      roomId,
      selectedIds: Array.from(this.pendingSelectionUpdate),
    };

    const sent = wsService.send('selection_change', payload);
    if (sent) {
      console.log('📤 Sent selection change:', payload);
    }

    this.pendingSelectionUpdate.clear();
  }

  // Event listeners
  onSelectionUpdated(callback: (selections: ElementSelection[]) => void): () => void {
    this.selectionListeners.add(callback);
    return () => this.selectionListeners.delete(callback);
  }

  getRemoteSelections(): ElementSelection[] {
    return Array.from(this.remoteSelections.values());
  }

  getLocalSelection(): string[] {
    return Array.from(this.localSelection);
  }

  private setupMessageHandlers(): void {
    wsService.on('selection_updated', (payload: SelectionUpdatedPayload) => {
      const selection: ElementSelection = {
        userId: payload.userId,
        username: payload.username,
        color: payload.color,
        selectedIds: payload.selectedIds,
      };

      // Update remote selections
      if (payload.selectedIds.length > 0) {
        this.remoteSelections.set(payload.userId, selection);
      } else {
        // Empty selection means user deselected everything
        this.remoteSelections.delete(payload.userId);
      }

      // Notify listeners
      this.selectionListeners.forEach(listener => {
        try {
          listener(this.getRemoteSelections());
        } catch (error) {
          console.error('Error in selection_updated handler:', error);
        }
      });
    });

    // Clear remote selections when user leaves
    wsService.on('user_left', (payload: { userId: string }) => {
      this.remoteSelections.delete(payload.userId);
      this.selectionListeners.forEach(listener => {
        try {
          listener(this.getRemoteSelections());
        } catch (error) {
          console.error('Error in user_left handler:', error);
        }
      });
    });
  }
}

// Singleton instance
export const selectionService = new SelectionService();

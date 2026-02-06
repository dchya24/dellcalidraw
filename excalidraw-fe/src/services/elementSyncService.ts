import { wsService } from './websocket';
import { roomService } from './roomService';
import type {
  UpdateElementsPayload,
  ElementChanges,
  ElementsUpdatedPayload,
  Element,
} from '../types/websocket';

class ElementSyncService {
  private localElements: Map<string, Element> = new Map();
  private isSyncEnabled: boolean = false;
  private elementUpdateListeners: Set<(payload: ElementsUpdatedPayload) => void> = new Set();

  enableSync(): void {
    this.isSyncEnabled = true;
    this.setupMessageHandlers();
    console.log('✅ Element sync enabled');
  }

  disableSync(): void {
    this.isSyncEnabled = false;
    console.log('❌ Element sync disabled');
  }

  isSyncing(): boolean {
    return this.isSyncEnabled && roomService.isConnectedToRoom();
  }

  // Initialize local elements from Excalidraw
  initializeElements(elements: readonly any[]): void {
    this.localElements.clear();
    elements.forEach(el => {
      if (el.id) {
        this.localElements.set(el.id, this.convertToBackendElement(el));
      }
    });
  }

  // Calculate delta and send changes to backend
  sendChanges(currentElements: readonly any[]): void {
    if (!this.isSyncing()) {
      // Just update local cache
      this.updateLocalCache(currentElements);
      return;
    }

    const changes = this.calculateDelta(currentElements);

    // Check if there are actual changes
    if (!changes.added?.length && !changes.updated?.length && !changes.deleted?.length) {
      return;
    }

    const roomId = roomService.getCurrentRoom();
    if (!roomId) {
      console.warn('Cannot send element changes: not in a room');
      return;
    }

    const payload: UpdateElementsPayload = {
      roomId,
      changes,
    };

    const sent = wsService.send('update_elements', payload);
    if (sent) {
      console.log('📤 Sent element changes:', changes);
    }

    // Update local cache
    this.updateLocalCache(currentElements);
  }

  // Handle incoming element updates from other users
  onElementsUpdated(callback: (payload: ElementsUpdatedPayload) => void): () => void {
    this.elementUpdateListeners.add(callback);
    return () => this.elementUpdateListeners.delete(callback);
  }

  private setupMessageHandlers(): void {
    wsService.on('elements_updated', (payload: ElementsUpdatedPayload) => {
      console.log('📥 Received element updates from user:', payload.userId);

      // Update local cache
      if (payload.changes.added) {
        payload.changes.added.forEach(el => {
          if (el.id) this.localElements.set(el.id, el);
        });
      }
      if (payload.changes.updated) {
        payload.changes.updated.forEach(el => {
          if (el.id) this.localElements.set(el.id, el);
        });
      }
      if (payload.changes.deleted) {
        payload.changes.deleted.forEach(id => {
          this.localElements.delete(id);
        });
      }

      // Notify listeners
      this.elementUpdateListeners.forEach(listener => {
        try {
          listener(payload);
        } catch (error) {
          console.error('Error in elements_updated handler:', error);
        }
      });
    });
  }

  private calculateDelta(currentElements: readonly any[]): ElementChanges {
    const changes: ElementChanges = {
      added: [],
      updated: [],
      deleted: [],
    };

    const currentIds = new Set<string>();

    // Find added and updated elements
    currentElements.forEach(excalidrawEl => {
      if (!excalidrawEl.id) return;

      currentIds.add(excalidrawEl.id);
      const backendEl = this.convertToBackendElement(excalidrawEl);
      const existingEl = this.localElements.get(excalidrawEl.id);

      if (!existingEl) {
        // New element
        changes.added?.push(backendEl);
      } else if (this.elementsDiffer(existingEl, backendEl)) {
        // Updated element
        changes.updated?.push(backendEl);
      }
    });

    // Find deleted elements
    this.localElements.forEach((_, id) => {
      if (!currentIds.has(id)) {
        changes.deleted?.push(id);
      }
    });

    return changes;
  }

  private updateLocalCache(elements: readonly any[]): void {
    elements.forEach(el => {
      if (el.id) {
        this.localElements.set(el.id, this.convertToBackendElement(el));
      }
    });
  }

  private convertToBackendElement(excalidrawEl: any): Element {
    return {
      id: excalidrawEl.id,
      type: excalidrawEl.type,
      x: excalidrawEl.x || 0,
      y: excalidrawEl.y || 0,
      width: excalidrawEl.width,
      height: excalidrawEl.height,
      angle: excalidrawEl.angle,
      stroke: excalidrawEl.strokeColor,
      background: excalidrawEl.backgroundColor,
      fill: excalidrawEl.fillStyle,
      data: {
        // Store additional Excalidraw-specific data
        version: excalidrawEl.version,
        versionNonce: excalidrawEl.versionNonce,
        isDeleted: excalidrawEl.isDeleted,
        seed: excalidrawEl.seed,
        groupIds: excalidrawEl.groupIds,
        frameId: excalidrawEl.frameId,
        index: excalidrawEl.index,
        roundness: excalidrawEl.roundness,
        boundElements: excalidrawEl.boundElements,
      },
    };
  }

  private convertToExcalidrawElement(backendEl: Element): any {
    return {
      id: backendEl.id,
      type: backendEl.type,
      x: backendEl.x,
      y: backendEl.y,
      width: backendEl.width,
      height: backendEl.height,
      angle: backendEl.angle,
      strokeColor: backendEl.stroke,
      backgroundColor: backendEl.background,
      fillStyle: backendEl.fill,
      ...(backendEl.data || {}),
    };
  }

  private elementsDiffer(el1: Element, el2: Element): boolean {
    // Compare key properties
    return (
      el1.type !== el2.type ||
      el1.x !== el2.x ||
      el1.y !== el2.y ||
      el1.width !== el2.width ||
      el1.height !== el2.height ||
      el1.angle !== el2.angle ||
      el1.stroke !== el2.stroke ||
      el1.background !== el2.background ||
      el1.fill !== el2.fill
    );
  }

  // Public method to get converted elements for Excalidraw
  getExcalidrawElements(): any[] {
    return Array.from(this.localElements.values()).map(el =>
      this.convertToExcalidrawElement(el)
    );
  }
}

// Singleton instance
export const elementSyncService = new ElementSyncService();

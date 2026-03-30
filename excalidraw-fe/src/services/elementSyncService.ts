import { wsService } from './websocket';
import { roomService } from './roomService';
import type {
  UpdateElementsPayload,
  ElementChanges,
  ElementsUpdatedPayload,
  ExcalidrawElementPayload,
} from '../types/websocket';
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types';
import { debounce } from '../utils/debounce';

class ElementSyncService {
  private localElements: Map<string, ExcalidrawElementPayload> = new Map();
  private isSyncEnabled: boolean = false;
  private elementUpdateListeners: Set<(payload: ElementsUpdatedPayload) => void> = new Set();
  private pendingChanges: ElementChanges | null = null;
  private debouncedSend: (() => void) | null = null;

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
  initializeElements(elements: readonly OrderedExcalidrawElement[]): void {
    this.localElements.clear();
    elements.forEach(el => {
      if (el.id) {
        this.localElements.set(el.id, this.convertToBackendElement(el));
      }
    });
  }

  // Calculate delta and send changes to backend
  sendChanges(currentElements: readonly OrderedExcalidrawElement[]): void {
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

    // Update local cache immediately
    this.updateLocalCache(currentElements);

    // Accumulate changes
    this.accumulateChanges(changes);

    // Debounce the send to avoid rate limiting
    if (!this.debouncedSend) {
      this.debouncedSend = debounce(() => {
        this.flushChanges();
      }, 100); // 100ms debounce
    }

    this.debouncedSend();
  }

  private accumulateChanges(changes: ElementChanges): void {
    if (!this.pendingChanges) {
      this.pendingChanges = { added: [], updated: [], deleted: [] };
    }

    // Merge added elements (avoid duplicates)
    if (changes.added) {
      const existingIds = new Set(this.pendingChanges.added?.map(e => e.id) || []);
      changes.added.forEach(el => {
        if (el.id && !existingIds.has(el.id)) {
          this.pendingChanges!.added?.push(el);
        }
      });
    }

    // Merge updated elements (latest wins)
    if (changes.updated) {
      const updatedMap = new Map(this.pendingChanges.updated?.map(e => [e.id, e]) || []);
      changes.updated.forEach(el => {
        if (el.id) {
          updatedMap.set(el.id, el);
        }
      });
      this.pendingChanges.updated = Array.from(updatedMap.values());
    }

    // Merge deleted elements
    if (changes.deleted) {
      const deletedSet = new Set(this.pendingChanges.deleted || []);
      changes.deleted.forEach(id => deletedSet.add(id));
      this.pendingChanges.deleted = Array.from(deletedSet);
    }
  }

  private flushChanges(): void {
    if (!this.pendingChanges) return;

    const changes = this.pendingChanges;
    this.pendingChanges = null;

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

  private calculateDelta(currentElements: readonly OrderedExcalidrawElement[]): ElementChanges {
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

  private updateLocalCache(elements: readonly OrderedExcalidrawElement[]): void {
    elements.forEach(el => {
      if (el.id) {
        this.localElements.set(el.id, this.convertToBackendElement(el));
      }
    });
  }

  private convertToBackendElement(excalidrawEl: OrderedExcalidrawElement): ExcalidrawElementPayload {
    return {
      id: excalidrawEl.id,
      type: excalidrawEl.type,
      x: excalidrawEl.x || 0,
      y: excalidrawEl.y || 0,
      width: excalidrawEl.width,
      height: excalidrawEl.height,
      angle: excalidrawEl.angle,
      strokeColor: excalidrawEl.strokeColor,
      backgroundColor: excalidrawEl.backgroundColor,
      fillStyle: excalidrawEl.fillStyle,
      strokeWidth: excalidrawEl.strokeWidth,
      strokeStyle: excalidrawEl.strokeStyle,
      roughness: excalidrawEl.roughness,
      opacity: excalidrawEl.opacity,
      seed: excalidrawEl.seed,
      version: excalidrawEl.version,
      versionNonce: excalidrawEl.versionNonce,
      isDeleted: excalidrawEl.isDeleted,
      groupIds: [...excalidrawEl.groupIds],
      frameId: excalidrawEl.frameId,
      boundElements: excalidrawEl.boundElements ? [...excalidrawEl.boundElements] : undefined,
      updated: excalidrawEl.updated,
      link: excalidrawEl.link,
      locked: excalidrawEl.locked,
    };
  }

  private convertToExcalidrawElement(backendEl: ExcalidrawElementPayload): OrderedExcalidrawElement {
    // Generate defaults for required fields
    const seed = backendEl.seed ?? Math.floor(Math.random() * 1000000);
    const version = backendEl.version ?? 1;
    const versionNonce = backendEl.versionNonce ?? Math.floor(Math.random() * 1000000);
    const updated = backendEl.updated ?? Date.now();

    return {
      id: backendEl.id,
      type: backendEl.type as OrderedExcalidrawElement["type"],
      x: backendEl.x,
      y: backendEl.y,
      width: backendEl.width ?? 0,
      height: backendEl.height ?? 0,
      angle: backendEl.angle ?? 0,
      strokeColor: backendEl.strokeColor ?? "#000000",
      backgroundColor: backendEl.backgroundColor ?? "transparent",
      fillStyle: (backendEl.fillStyle ?? "solid") as OrderedExcalidrawElement["fillStyle"],
      strokeWidth: backendEl.strokeWidth ?? 1,
      strokeStyle: (backendEl.strokeStyle ?? "solid") as OrderedExcalidrawElement["strokeStyle"],
      roughness: backendEl.roughness ?? 1,
      opacity: backendEl.opacity ?? 100,
      seed,
      version,
      versionNonce,
      index: null, // Will be set by Excalidraw
      isDeleted: backendEl.isDeleted ?? false,
      groupIds: backendEl.groupIds ?? [],
      frameId: backendEl.frameId ?? null,
      boundElements: backendEl.boundElements ?? null,
      updated,
      link: backendEl.link ?? null,
      locked: backendEl.locked ?? false,
      // Add roundness default
      roundness: null as OrderedExcalidrawElement["roundness"],
      // Merge any additional data
      ...(backendEl.data || {}),
    } as OrderedExcalidrawElement;
  }

  private elementsDiffer(el1: ExcalidrawElementPayload, el2: ExcalidrawElementPayload): boolean {
    // Compare key properties for change detection
    return (
      el1.type !== el2.type ||
      el1.x !== el2.x ||
      el1.y !== el2.y ||
      el1.width !== el2.width ||
      el1.height !== el2.height ||
      el1.angle !== el2.angle ||
      el1.strokeColor !== el2.strokeColor ||
      el1.backgroundColor !== el2.backgroundColor ||
      el1.fillStyle !== el2.fillStyle ||
      el1.strokeWidth !== el2.strokeWidth ||
      el1.strokeStyle !== el2.strokeStyle ||
      el1.roughness !== el2.roughness ||
      el1.opacity !== el2.opacity ||
      el1.version !== el2.version ||
      el1.versionNonce !== el2.versionNonce ||
      el1.isDeleted !== el2.isDeleted ||
      el1.locked !== el2.locked
    );
  }

  // Public method to get converted elements for Excalidraw
  getExcalidrawElements(): OrderedExcalidrawElement[] {
    return Array.from(this.localElements.values()).map(el =>
      this.convertToExcalidrawElement(el)
    );
  }
}

// Singleton instance
export const elementSyncService = new ElementSyncService();

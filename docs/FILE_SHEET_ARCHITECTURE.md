# Frontend File & Sheet Architecture

**Created:** 2026-05-13
**Updated:** 2026-05-13
**Status:** ✅ Phase A-D Complete
**Focus:** Frontend-only (localStorage), backend sync nanti

---

## Overview

Dokumen ini menjelaskan arsitektur file dan sheet di frontend, mapping ke ERD backend, dan rencana pengerjaan selanjutnya.

---

## Current State (After Store Merge)

### Single Store: `useWhiteboardStore`

```
whiteboard-storage (localStorage)
└── files: WhiteboardFile[]
    ├── WhiteboardFile
    │   ├── id: string
    │   ├── name: string
    │   ├── isCloud: boolean
    │   ├── cloudId?: string
    │   ├── tabs: WhiteboardTab[]
    │   │   ├── WhiteboardTab (= 1 Sheet/Scene)
    │   │   │   ├── id: string
    │   │   │   ├── title: string ("Sheet 1")
    │   │   │   ├── roomId: string (unique, untuk collab)
    │   │   │   └── data: { elements[], appState, files }
    │   │   └── ...more tabs
    │   ├── activeTabId: string
    │   ├── createdAt: number
    │   └── lastModified: number
    └── ...more files
```

### Mapping ke ERD Backend

| Frontend (localStorage) | Backend (PostgreSQL) | Relasi |
|---|---|---|
| `WhiteboardFile` | `user_files` | 1:1 |
| `WhiteboardFile.name` | `user_files.name` | - |
| `WhiteboardFile.tabs.length` | `user_files.tab_count` | metadata only |
| `WhiteboardTab` | `rooms` | 1 tab = 1 room |
| `WhiteboardTab.roomId` | `rooms.key` | unique identifier |
| `WhiteboardTab.title` | `rooms.name` | - |
| `WhiteboardTab.data.elements[]` | `room_elements` | 1 room : N elements |
| `WhiteboardTab.data.files` | `room_files` | binary assets |

### Yang Belum Terhubung (Backend)

- `rooms` belum punya `file_id` FK ke `user_files` — backend tidak tahu tab mana milik file mana
- `user_files.tab_count` hanya angka, bukan referensi ke rooms
- Saat create file + default sheet, backend tidak otomatis create room

---

## Completed Work

### ✅ Store Merge (2026-05-13)

**Problem:** Dua store (`useFileStore` + `useWhiteboardStore`) menyimpan data duplikat di localStorage (`file-storage` + `whiteboard-storage`), menyebabkan konflik dan out-of-sync.

**Solution:** Gabung jadi satu store `useWhiteboardStore` dengan satu key `whiteboard-storage`.

**Changes:**
- `src/store/useWhiteboardStore.ts` — ditambahkan `isCloud`, `cloudId`, `isLoading`, `syncStatus`, cloud operations
- `src/components/Sidebar.tsx` — pakai `useWhiteboardStore` langsung
- `src/App.tsx` — import diganti ke `useWhiteboardStore`
- `src/store/useFileStore.ts` — **dihapus**

---

## Current Export/Import Behavior

| Feature | Scope | Keterangan |
|---|---|---|
| Export JSON (.excalidraw) | 1 sheet aktif | Dari `excalidrawAPI.getSceneElements()` |
| Export PNG | 1 sheet aktif | - |
| Export SVG | 1 sheet aktif | - |
| Import native .excalidraw | 1 sheet aktif | Load ke tab aktif |
| Import multi-tab format | Semua sheets | Replace semua tabs di file aktif |
| Save to Cloud | 1 sheet aktif | By `roomId` → `room_elements` |
| Load from Cloud | 1 sheet aktif | By `roomId` → `room_elements` |
| Drag & Drop (.excalidraw) | Buat tab baru | - |
| Drag & Drop (multi-tab) | Semua sheets | Replace tabs di file aktif |

---

## Planned Work (FE Only)

### Phase A: File Creation with Default Sheet ✅

Saat create file baru, otomatis buat 1 default sheet ("Sheet 1") dengan `roomId` unik.

**Status:** Sudah implemented di `createEmptyFile()`.

---

### Phase B: Export/Import per File (All Sheets) ✅

**Goal:** Tambah opsi export/import seluruh file (semua sheets sekaligus).

**Implemented in:** `src/services/exportImportService.ts`

Service layer reusable yang bisa dipanggil dari komponen mana saja (Toolbar, MainMenu, Sidebar, dll).

**Functions:**
- `exportActiveSheetJSON()` — export 1 sheet sebagai .excalidraw
- `exportFileAllSheets()` — export semua sheets sebagai .dellcalidraw
- `exportActiveSheetPNG()` / `exportActiveSheetSVG()` — export gambar
- `parseImportData()` — detect format file
- `handleFileImport()` — auto-import berdasarkan format
- `importDellcalidrawFile()` — import multi-sheet ke file aktif (replace)
- `importDellcalidrawAsNewFile()` — import multi-sheet sebagai file baru
- `importExcalidrawNative()` / `importElementsArray()` — import single scene
- `saveActiveSheetToCloud()` / `loadActiveSheetFromCloud()` — cloud per sheet
- `saveAllSheetsToCloud()` / `loadAllSheetsFromCloud()` — cloud per file

#### Export All Sheets ✅
```typescript
// Format: .dellcalidraw (custom JSON)
{
  type: "dellcalidraw",
  version: 1,
  name: "My File",
  tabs: [
    {
      id: "...",
      title: "Sheet 1",
      roomId: "...",
      data: { elements: [...], appState: {...}, files: {...} }
    },
    {
      id: "...",
      title: "Sheet 2",
      ...
    }
  ],
  activeTabId: "...",
  exportedAt: "2026-05-13T..."
}
```

#### Import File ✅
- Detect format: `.dellcalidraw` (multi-sheet) vs `.excalidraw` (single scene)
- Multi-sheet: buat file baru atau replace file aktif
- Single scene: load ke sheet aktif
- Drag & drop support untuk kedua format

#### UI Changes ✅
- Toolbar export dropdown: "Export Sheet (.excalidraw)" + "Export File (all sheets)"
- Import: auto-detect format
- Whiteboard MainMenu: "Save To File" exports all sheets
- Drag & drop: `.dellcalidraw` creates new file, `.excalidraw` creates new tab

---

### Phase C: Save/Load Cloud per File (All Sheets)

**Goal:** Save/load semua sheets dalam 1 file ke cloud sekaligus.

> ⚠️ **Blocked by backend** — perlu `file_id` di tabel `rooms` atau endpoint baru.
> Untuk sekarang, tetap save/load per sheet (per room).

#### Workaround (FE-only, tanpa backend change):
- Loop semua tabs dalam file aktif
- Save each tab's elements ke masing-masing room (`POST /api/rooms/:roomId/canvas/save`)
- Load each tab's elements dari masing-masing room (`GET /api/rooms/:roomId/canvas/load`)

```typescript
// Save all sheets in active file
const saveAllSheets = async () => {
  const file = getActiveFile();
  if (!file) return;
  
  for (const tab of file.tabs) {
    await apiService.saveCanvas(tab.roomId);
  }
};

// Load all sheets in active file  
const loadAllSheets = async () => {
  const file = getActiveFile();
  if (!file) return;
  
  for (const tab of file.tabs) {
    const result = await apiService.loadCanvas(tab.roomId);
    if (result.elements?.length > 0) {
      saveTabState(tab.id, result.elements, {}, {});
    }
  }
};
```

---

### Phase D: Sidebar UX Improvements ✅

- [x] Search/filter files by name
- [x] Sort files: by name (A-Z, Z-A), by date (newest, oldest), by sheet count
- [x] Duplicate file (clone all sheets with new IDs)
- [x] Export file button per item
- [x] Relative timestamps ("5m ago", "2h ago", "3d ago")
- [x] Better empty states (no results vs no files)
- [x] File count with filter info ("3 of 7 files")

---

### Phase E: Sheet Management UX

- [ ] Drag-to-reorder sheets (tabs)
- [ ] Duplicate sheet (clone tab with elements)
- [ ] Move sheet to another file
- [ ] Sheet color/icon indicator
- [ ] Sheet preview thumbnail

---

## Priority Order

| # | Phase | Effort | Impact | Status |
|---|---|---|---|---|
| 1 | A: Default sheet on file create | ✅ Done | High | Complete |
| 2 | B: Export/Import per file | ✅ Done | High | Complete |
| 3 | D: Sidebar UX | ✅ Done | Medium | Complete |
| 4 | C: Save/Load cloud per file | Medium (1 day FE) | High | Backend workaround ready |
| 5 | E: Sheet management UX | Medium (2-3 days) | Medium | Pending |

---

## Technical Notes

### localStorage Size Limit

- Browser limit: ~5-10MB per origin
- Excalidraw elements bisa besar (terutama dengan banyak text/freehand)
- Monitor: jika file besar, pertimbangkan IndexedDB migration
- Zustand persist middleware support custom storage adapter

### Room ID Generation

- Setiap sheet punya `roomId` unik (nanoid 10 chars)
- `roomId` dipakai untuk:
  - WebSocket collaboration (join room)
  - Cloud save/load (`/api/rooms/:roomId/canvas/save`)
  - URL sharing (`?room=abc1234567`)

### File ID vs Cloud ID

- `file.id` — local identifier (nanoid atau UUID dari backend)
- `file.cloudId` — backend `user_files.id` (hanya ada jika authenticated & synced)
- Saat not authenticated: `cloudId` = undefined, semua local

---

## Related Docs

- [ERD](../erd/ERD.md) — Database schema & relationships
- [Frontend Integration](./FRONTEND_INTEGRATION.md) — Phase 3 & 4 implementation
- [Phase Summary](./PHASE_SUMMARY.md) — All completed phases
- [Pending Development](./PENDING_DEVELOPMENT.md) — Future features backlog

# Project Context: Hand-Drawn Whiteboard (Excalidraw + Multi-Tab)

## 🎯 Project Vision
Build a high-performance, "vibe coding" friendly whiteboard application using **React** and **@excalidraw/excalidraw**.
The application must feel seamless and include advanced features typically found in spreadsheet apps: **Multi-Tab/Sheet management** and **Real-time Collaboration**.

## 🛠 Tech Stack
- **Framework:** React (Vite) + TypeScript
- **Styling:** Tailwind CSS (for UI overlay, Sidebar, Tabs)
- **Canvas Engine:** `@excalidraw/excalidraw`
- **State Management:** Zustand (Crucial for handling Multi-Tab state)
- **Icons:** Lucide-React
- **Collaboration (Backend):** Socket.io or Yjs (for CRDT syncing)
- **Utils:** `nanoid` (for generating Tab IDs and Room IDs), `lodash.debounce`

---

## 🏗 Architecture & Data Model (Zustand Store)
**IMPORTANT:** The most complex feature is the **Multi-Tab System**. The agent must handle state switching carefully to avoid data loss.

### Tab State Structure
Use this data shape to manage multiple drawings within the global store:

```typescript
type TabId = string;

interface WhiteboardTab {
  id: TabId;
  title: string;
  // Snapshot of the canvas data for this tab
  data: {
    elements: readonly any[]; // ExcalidrawElement[]
    appState: Partial<any>;   // AppState
    files: any;               // BinaryFiles
  };
  lastModified: number;
}

interface AppStore {
  tabs: WhiteboardTab[];
  activeTabId: TabId;
  // Actions
  addTab: () => void;
  removeTab: (id: TabId) => void;
  renameTab: (id: TabId, newTitle: string) => void;
  setActiveTab: (id: TabId) => void;
  // Sync current canvas state to the store before switching
  saveTabState: (id: TabId, elements: any, appState: any, files: any) => void;
}

## Implementation Roadmap (Step-by-Step)

### Phase 1: Foundation & Basic Canvas
[ ] Setup Vite + React + TS + Tailwind.

[ ] Install dependencies: @excalidraw/excalidraw, lucide-react, zustand, nanoid.

[ ] Buat komponen Whiteboard.tsx yang membungkus Excalidraw secara fullscreen.

[ ] Implementasi excalidrawAPI ref untuk kontrol programatik.

### Phase 2: Multi-Tab Management (Core Logic)
[ ] Buat Zustand Store sesuai schema Data Model.

[ ] Implementasi TabBar.tsx UI (letakkan di atas atau bawah canvas).

[ ] Tab Switching Logic:

Sebelum pindah, simpan state canvas saat ini ke Store.

Ubah activeTabId.

Load data tab baru ke canvas via excalidrawAPI.updateScene().

Reset Undo History: Panggil api.history.clear() agar undo tidak cross-tab.

[ ] Fitur Add, Rename, dan Delete tab.

### Phase 3: Persistence & File Operations
[ ] Auto-save: Implementasi localStorage sync agar data tidak hilang saat refresh (Debounced).

[ ] Export Feature: Tombol untuk download sebagai .excalidraw (JSON), PNG, dan SVG.

[ ] Import Feature: Drag-and-drop file JSON untuk membuka project lama.

### Phase 4: Real-time Collaboration Setup
[ ] Setup koneksi Socket.io atau provider realtime.

[ ] Implementasi Room System: Setiap tab bisa memiliki Room ID unik.

[ ] Syncing: Kirim perubahan element (debounced) ke peer lain.

[ ] User Awareness: Tampilkan kursor user lain dan inisial nama mereka di canvas.

### Phase 5: UI/UX Polish & Advance Features
[ ] Custom Sidebar untuk manajemen aset atau daftar tab yang lebih rapi.

[ ] Toggle Dark/Light mode yang sinkron antara UI Tailwind dan Canvas Excalidraw.

[ ] Optimasi performa untuk canvas dengan ribuan elemen.

📝 Workflow & Documentation (MANDATORY)
Setiap kali kamu menyelesaikan satu Phase dari Roadmap di atas, kamu wajib membuat rangkuman dalam format Markdown.

Instruksi:

Buat atau update file bernama PHASE_SUMMARY.md.

Gunakan template berikut untuk setiap fase yang selesai:

Markdown

## ✅ Phase [X]: [Nama Phase] Completed
**Date:** [YYYY-MM-DD]

### 🛠 Features Implemented
- [List fitur yang berhasil dibuat]
- [List komponen baru yang dibuat]

### 🧠 Technical Decisions & Challenges
- [Jelaskan keputusan teknis penting, misal: struktur state yang dipilih]
- [Jelaskan kendala yang dihadapi dan solusinya]

### ⚠️ Known Issues / TODOs
- [Bug yang belum fixed atau fitur yang ditunda]

### ⏭️ Next Steps
- [Rencana untuk fase berikutnya]
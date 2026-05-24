# Guest → Cloud Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When a guest user logs in/registers, their local localStorage files (with all sheets and elements) are preserved and migrated to the cloud, not replaced.

**Architecture:** 
1. Frontend: Modify `loadFiles()` to merge local + cloud instead of replacing. Add a migration helper that uploads local-only files to the backend.
2. Backend: Add a new bulk migration endpoint `POST /api/files/migrate` that creates files with full tab data (elements + appState per tab) in a single request.
3. Backend: Add a new DB migration for `file_tabs` table to persist per-tab canvas data.

**Tech Stack:** Go (chi), PostgreSQL, React (Zustand), TypeScript

---

## Task 1: Backend — Add `file_tabs` table migration

**Files:**
- Create: `excalidraw-be/internal/database/migrations/000008_file_tabs.up.sql`
- Create: `excalidraw-be/internal/database/migrations/000008_file_tabs.down.sql`

**Step 1: Create the up migration**

```sql
-- file_tabs: stores per-tab canvas data (elements, appState) for user files
CREATE TABLE IF NOT EXISTS file_tabs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id     UUID NOT NULL REFERENCES user_files(id) ON DELETE CASCADE,
    tab_key     VARCHAR(255) NOT NULL,
    title       VARCHAR(255) NOT NULL DEFAULT 'Sheet 1',
    room_id     VARCHAR(50),
    elements    JSONB NOT NULL DEFAULT '[]',
    app_state   JSONB NOT NULL DEFAULT '{}',
    files_data  JSONB NOT NULL DEFAULT '{}',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_tabs_file_id ON file_tabs(file_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_file_tabs_file_id_tab_key ON file_tabs(file_id, tab_key);
```

**Step 2: Create the down migration**

```sql
DROP TABLE IF EXISTS file_tabs;
```

**Step 3: Commit**

```bash
git add excalidraw-be/internal/database/migrations/000008_file_tabs.*
git commit -m "feat(be): add file_tabs migration for per-tab canvas persistence"
```

---

## Task 2: Backend — Add `file_tabs` database repository methods

**Files:**
- Create: `excalidraw-be/internal/database/file_tabs.go`

**Step 1: Create the file_tabs repository**

```go
package database

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

type FileTab struct {
	ID        string
	FileID    string
	TabKey    string
	Title     string
	RoomID    string
	Elements  json.RawMessage
	AppState  json.RawMessage
	FilesData json.RawMessage
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

// CreateFileTab inserts a new tab for a file
func (p *PostgresClient) CreateFileTab(fileID, tabKey, title, roomID string, elements, appState, filesData json.RawMessage, sortOrder int) (*FileTab, error) {
	if elements == nil {
		elements = json.RawMessage(`[]`)
	}
	if appState == nil {
		appState = json.RawMessage(`{}`)
	}
	if filesData == nil {
		filesData = json.RawMessage(`{}`)
	}

	var tab FileTab
	err := p.db.QueryRow(
		`INSERT INTO file_tabs (file_id, tab_key, title, room_id, elements, app_state, files_data, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, file_id, tab_key, title, room_id, elements, app_state, files_data, sort_order, created_at::text, updated_at::text`,
		fileID, tabKey, title, roomID, elements, appState, filesData, sortOrder,
	).Scan(&tab.ID, &tab.FileID, &tab.TabKey, &tab.Title, &tab.RoomID, &tab.Elements, &tab.AppState, &tab.FilesData, &tab.SortOrder, &tab.CreatedAt, &tab.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create file tab: %w", err)
	}
	return &tab, nil
}

// GetFileTabs returns all tabs for a file, ordered by sort_order
func (p *PostgresClient) GetFileTabs(fileID string) ([]FileTab, error) {
	rows, err := p.db.Query(
		`SELECT id, file_id, tab_key, title, room_id, elements, app_state, files_data, sort_order, created_at::text, updated_at::text
		 FROM file_tabs WHERE file_id = $1 ORDER BY sort_order ASC`,
		fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file tabs: %w", err)
	}
	defer rows.Close()

	var tabs []FileTab
	for rows.Next() {
		var tab FileTab
		if err := rows.Scan(&tab.ID, &tab.FileID, &tab.TabKey, &tab.Title, &tab.RoomID, &tab.Elements, &tab.AppState, &tab.FilesData, &tab.SortOrder, &tab.CreatedAt, &tab.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan file tab: %w", err)
		}
		tabs = append(tabs, tab)
	}
	return tabs, rows.Err()
}

// UpdateFileTab updates a tab's canvas data
func (p *PostgresClient) UpdateFileTab(tabID string, elements, appState, filesData json.RawMessage) error {
	if elements == nil {
		elements = json.RawMessage(`[]`)
	}
	if appState == nil {
		appState = json.RawMessage(`{}`)
	}
	if filesData == nil {
		filesData = json.RawMessage(`{}`)
	}

	_, err := p.db.Exec(
		`UPDATE file_tabs SET elements = $2, app_state = $3, files_data = $4, updated_at = NOW()
		 WHERE id = $1`,
		tabID, elements, appState, filesData,
	)
	if err != nil {
		return fmt.Errorf("failed to update file tab: %w", err)
	}
	slog.Debug("File tab updated", "tabID", tabID)
	return nil
}

// DeleteFileTabsByFileID deletes all tabs for a file
func (p *PostgresClient) DeleteFileTabsByFileID(fileID string) error {
	_, err := p.db.Exec(`DELETE FROM file_tabs WHERE file_id = $1`, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file tabs: %w", err)
	}
	return nil
}
```

**Step 2: Commit**

```bash
git add excalidraw-be/internal/database/file_tabs.go
git commit -m "feat(be): add file_tabs repository methods"
```

---

## Task 3: Backend — Add bulk migrate endpoint

**Files:**
- Modify: `excalidraw-be/cmd/server/file_management_handlers.go`

**Step 1: Add the `MigrateLocalFiles` handler**

Add the following to `file_management_handlers.go`:

```go
// MigrateLocalFiles bulk-creates files with tabs from local storage
// POST /api/files/migrate
func (h *FileManagementHandler) MigrateLocalFiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	var req struct {
		Files []struct {
			Name      string `json:"name"`
			ActiveTab string `json:"activeTabId"`
			Tabs      []struct {
				Title    string          `json:"title"`
				RoomID   string          `json:"roomId"`
				Elements json.RawMessage `json:"elements"`
				AppState json.RawMessage `json:"appState"`
				Files    json.RawMessage `json:"files"`
			} `json:"tabs"`
		} `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if len(req.Files) == 0 {
		writeJSONError(w, http.StatusBadRequest, "No files to migrate", "empty_files")
		return
	}

	type MigratedTab struct {
		TabKey    string          `json:"tabKey"`
		Title     string          `json:"title"`
		RoomID    string          `json:"roomId"`
		Elements  json.RawMessage `json:"elements"`
		AppState  json.RawMessage `json:"appState"`
		FilesData json.RawMessage `json:"files"`
	}

	type MigratedFile struct {
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		ActiveTab string          `json:"activeTabId"`
		Tabs      []MigratedTab   `json:"tabs"`
	}

	migratedFiles := make([]MigratedFile, 0, len(req.Files))

	for _, f := range req.Files {
		// Create the file record
		file, err := h.db.CreateUserFile(userID, f.Name)
		if err != nil {
			slog.Error("Failed to create file during migration", "error", err, "userID", userID)
			continue
		}

		migrated := MigratedFile{
			ID:        file.ID,
			Name:      file.Name,
			ActiveTab: f.ActiveTab,
			Tabs:      make([]MigratedTab, 0, len(f.Tabs)),
		}

		// Create tabs
		for i, t := range f.Tabs {
			tabKey := fmt.Sprintf("tab_%d", i)
			elements := t.Elements
			if elements == nil {
				elements = json.RawMessage(`[]`)
			}
			appState := t.AppState
			if appState == nil {
				appState = json.RawMessage(`{}`)
			}
			filesData := t.Files
			if filesData == nil {
				filesData = json.RawMessage(`{}`)
			}

			tab, err := h.db.CreateFileTab(file.ID, tabKey, t.Title, t.RoomID, elements, appState, filesData, i)
			if err != nil {
				slog.Error("Failed to create tab during migration", "error", err, "fileID", file.ID)
				continue
			}

			migrated.Tabs = append(migrated.Tabs, MigratedTab{
				TabKey:    tab.TabKey,
				Title:     tab.Title,
				RoomID:    tab.RoomID,
				Elements:  tab.Elements,
				AppState:  tab.AppState,
				FilesData: tab.FilesData,
			})
		}

		// Update tab count
		_, _ = h.db.UpdateUserFile(file.ID, file.Name, len(f.Tabs))

		migratedFiles = append(migratedFiles, migrated)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   migratedFiles,
		"count":   len(migratedFiles),
	})
}
```

Don't forget to add `"fmt"` to imports if not already present.

**Step 2: Register the route in `main.go`**

Add this route inside the authenticated group (after the existing file management routes):

```go
r.Post("/api/files/migrate", fileMgmtHandler.MigrateLocalFiles)
```

**Step 3: Commit**

```bash
git add excalidraw-be/cmd/server/file_management_handlers.go excalidraw-be/cmd/server/main.go
git commit -m "feat(be): add bulk file migration endpoint for guest-to-cloud sync"
```

---

## Task 4: Backend — Modify ListUserFiles to include tabs data

**Files:**
- Modify: `excalidraw-be/cmd/server/file_management_handlers.go`

**Step 1: Update `ListUserFiles` to include tabs**

Replace the `ListUserFiles` method with one that also fetches tabs per file:

```go
// ListUserFiles returns all files with tabs for the authenticated user
// GET /api/files
func (h *FileManagementHandler) ListUserFiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	files, err := h.db.GetUserFiles(userID)
	if err != nil {
		slog.Error("Failed to list user files", "error", err, "userID", userID)
		writeJSONError(w, http.StatusInternalServerError, "Failed to list files", "list_failed")
		return
	}

	type TabResponse struct {
		TabKey    string          `json:"tabKey"`
		Title     string          `json:"title"`
		RoomID    string          `json:"roomId"`
		Elements  json.RawMessage `json:"elements"`
		AppState  json.RawMessage `json:"appState"`
		FilesData json.RawMessage `json:"files"`
	}

	type FileWithTabs struct {
		ID        string        `json:"id"`
		UserID    string        `json:"userId"`
		Name      string        `json:"name"`
		TabCount  int           `json:"tabCount"`
		CreatedAt string        `json:"createdAt"`
		UpdatedAt string        `json:"updatedAt"`
		Tabs      []TabResponse `json:"tabs"`
	}

	result := make([]FileWithTabs, 0, len(files))
	for _, f := range files {
		tabs, err := h.db.GetFileTabs(f.ID)
		fileWithTabs := FileWithTabs{
			ID:        f.ID,
			UserID:    f.UserID,
			Name:      f.Name,
			TabCount:  f.TabCount,
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.UpdatedAt,
		}

		if err == nil && len(tabs) > 0 {
			for _, t := range tabs {
				fileWithTabs.Tabs = append(fileWithTabs.Tabs, TabResponse{
					TabKey:    t.TabKey,
					Title:     t.Title,
					RoomID:    t.RoomID,
					Elements:  t.Elements,
					AppState:  t.AppState,
					FilesData: t.FilesData,
				})
			}
		}
		fileWithTabs.TabCount = len(fileWithTabs.Tabs)

		result = append(result, fileWithTabs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   result,
		"count":   len(result),
	})
}
```

**Step 2: Commit**

```bash
git add excalidraw-be/cmd/server/file_management_handlers.go
git commit -m "feat(be): include tabs data in ListUserFiles response"
```

---

## Task 5: Frontend — Add migration service method

**Files:**
- Modify: `excalidraw-fe/src/services/fileService.ts`

**Step 1: Add the `migrateFiles` method and update `UserFile` type**

Add the following to `fileService.ts`:

1. Update the `UserFile` type:
```typescript
export interface FileTab {
  tabKey: string;
  title: string;
  roomId: string;
  elements: unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
}

export interface UserFile {
  id: string;
  userId: string;
  name: string;
  tabCount: number;
  createdAt: string;
  updatedAt: string;
  tabs?: FileTab[];
}
```

2. Add the `migrateFiles` method to the class:
```typescript
async migrateFiles(files: Array<{
  name: string;
  activeTabId: string;
  tabs: Array<{
    title: string;
    roomId: string;
    elements: unknown[];
    appState: Record<string, unknown>;
    files: Record<string, unknown>;
  }>;
}>): Promise<{ files: Array<UserFile & { tabs: FileTab[] }>; count: number }> {
  return this.request("/api/files/migrate", {
    method: "POST",
    body: JSON.stringify({ files }),
  });
}
```

**Step 2: Commit**

```bash
git add excalidraw-fe/src/services/fileService.ts
git commit -m "feat(fe): add file migration service method"
```

---

## Task 6: Frontend — Modify `loadFiles()` to merge + migrate

**Files:**
- Modify: `excalidraw-fe/src/store/useWhiteboardStore.ts`

This is the core change. The `loadFiles()` function needs to:

1. Save current local files before loading cloud data
2. Fetch cloud files
3. Merge: keep local-only files, add cloud files
4. Migrate local-only files to cloud
5. Update local file IDs to cloud IDs after migration

**Step 1: Update the `loadFiles` method**

Replace the existing `loadFiles` implementation with:

```typescript
loadFiles: async () => {
  const authState = useAuthStore.getState();
  if (!authState.isAuthenticated) return;

  set({ isLoading: true, syncStatus: "syncing" as const });

  try {
    // 1. Save current local files before fetching cloud data
    const currentFiles = get().files;
    const localOnlyFiles = currentFiles.filter(f => !f.isCloud);

    // 2. Fetch cloud files
    const response = await fileService.listFiles();
    const cloudFiles: WhiteboardFile[] = response.files.map(
      (file: UserFile) => {
        // If cloud file has tabs data, use it
        const tabs = file.tabs && file.tabs.length > 0
          ? file.tabs.map((t, idx) => ({
              id: nanoid(),
              title: t.title,
              roomId: t.roomId || nanoid(10),
              data: {
                elements: (t.elements || []) as readonly OrderedExcalidrawElement[],
                appState: (t.appState || {}) as Partial<AppState>,
                files: (t.files || {}) as Record<string, unknown>,
              },
              lastModified: Date.now(),
            }))
          : [createEmptyTab("Sheet 1")];

        return {
          id: file.id,
          name: file.name,
          tabs,
          activeTabId: tabs[0].id,
          createdAt: new Date(file.createdAt).getTime(),
          lastModified: new Date(file.updatedAt).getTime(),
          isCloud: true,
          cloudId: file.id,
        };
      }
    );

    // 3. Merge: cloud files + local-only files
    const merged = [...cloudFiles, ...localOnlyFiles];

    set({
      files: merged.length > 0 ? merged : [createEmptyFile("Untitled")],
      isLoading: false,
      syncStatus: "synced" as const,
      lastSyncedAt: Date.now(),
    });

    // 4. Migrate local-only files to cloud in background
    if (localOnlyFiles.length > 0) {
      migrateLocalFiles(localOnlyFiles);
    }
  } catch (error) {
    console.error("Failed to sync from cloud:", error);
    // Keep existing data as fallback — don't overwrite
    set({ isLoading: false, syncStatus: "error" as const });
  }
},
```

**Step 2: Add the `migrateLocalFiles` helper function** (outside the store, above the `useAuthStore.subscribe` call)

```typescript
async function migrateLocalFiles(localFiles: WhiteboardFile[]) {
  try {
    const migrationPayload = localFiles.map(f => ({
      name: f.name,
      activeTabId: f.activeTabId,
      tabs: f.tabs.map(t => ({
        title: t.title,
        roomId: t.roomId,
        elements: t.data.elements,
        appState: t.data.appState,
        files: t.data.files,
      })),
    }));

    const result = await fileService.migrateFiles(migrationPayload);

    // After successful migration, update local state to mark files as cloud
    const state = useWhiteboardStore.getState();
    const updatedFiles = state.files.map(f => {
      const migratedFile = result.files.find(mf => mf.name === f.name);
      if (migratedFile && !f.isCloud) {
        // Reconstruct tabs with cloud data
        const tabs = migratedFile.tabs.map((t, idx) => ({
          ...f.tabs[idx] || createEmptyTab(t.title),
          roomId: t.roomId || f.tabs[idx]?.roomId || nanoid(10),
        }));
        return {
          ...f,
          id: migratedFile.id,
          isCloud: true,
          cloudId: migratedFile.id,
          tabs,
        };
      }
      return f;
    });

    useWhiteboardStore.setState({
      files: updatedFiles,
      syncStatus: "synced" as const,
      lastSyncedAt: Date.now(),
    });
  } catch (error) {
    console.error("Failed to migrate local files to cloud:", error);
    // Files stay as local-only, still usable
  }
}
```

**Step 3: Commit**

```bash
git add excalidraw-fe/src/store/useWhiteboardStore.ts
git commit -m "feat(fe): merge local + cloud files on login, migrate local to cloud"
```

---

## Task 7: Frontend — Save tab data to cloud when modifying tabs

**Files:**
- Modify: `excalidraw-fe/src/services/fileService.ts`

**Step 1: Add `saveFileTabs` method**

```typescript
async saveFileTabs(fileId: string, tabs: Array<{
  tabKey: string;
  title: string;
  roomId: string;
  elements: unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
}>): Promise<{ success: boolean }> {
  return this.request(`/api/files/${fileId}/tabs`, {
    method: "PUT",
    body: JSON.stringify({ tabs }),
  });
}
```

**Step 2: Commit**

```bash
git add excalidraw-fe/src/services/fileService.ts
git commit -m "feat(fe): add saveFileTabs service method"
```

---

## Task 8: Backend — Add endpoint to save tabs for a file

**Files:**
- Modify: `excalidraw-be/cmd/server/file_management_handlers.go`
- Modify: `excalidraw-be/cmd/server/main.go`

**Step 1: Add `SaveFileTabs` handler**

```go
// SaveFileTabs saves/updates all tabs for a file
// PUT /api/files/:fileId/tabs
func (h *FileManagementHandler) SaveFileTabs(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileId")
	if fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing fileId", "missing_file_id")
		return
	}

	// Check ownership
	existing, err := h.db.GetUserFile(fileID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "File not found", "file_not_found")
		return
	}
	if existing.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Access denied", "forbidden")
		return
	}

	var req struct {
		Tabs []struct {
			TabKey    string          `json:"tabKey"`
			Title     string          `json:"title"`
			RoomID    string          `json:"roomId"`
			Elements  json.RawMessage `json:"elements"`
			AppState  json.RawMessage `json:"appState"`
			Files     json.RawMessage `json:"files"`
		} `json:"tabs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	// Delete existing tabs and recreate
	if err := h.db.DeleteFileTabsByFileID(fileID); err != nil {
		slog.Error("Failed to clear existing tabs", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to save tabs", "save_failed")
		return
	}

	for i, t := range req.Tabs {
		elements := t.Elements
		if elements == nil {
			elements = json.RawMessage(`[]`)
		}
		appState := t.AppState
		if appState == nil {
			appState = json.RawMessage(`{}`)
		}
		filesData := t.Files
		if filesData == nil {
			filesData = json.RawMessage(`{}`)
		}

		tabKey := t.TabKey
		if tabKey == "" {
			tabKey = fmt.Sprintf("tab_%d", i)
		}

		_, err := h.db.CreateFileTab(fileID, tabKey, t.Title, t.RoomID, elements, appState, filesData, i)
		if err != nil {
			slog.Error("Failed to create tab", "error", err, "fileID", fileID)
			continue
		}
	}

	// Update tab count
	_, _ = h.db.UpdateUserFile(fileID, existing.Name, len(req.Tabs))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(req.Tabs),
	})
}
```

**Step 2: Register the route in `main.go`**

Add inside the authenticated group:
```go
r.Put("/api/files/{fileId}/tabs", fileMgmtHandler.SaveFileTabs)
```

**Step 3: Commit**

```bash
git add excalidraw-be/cmd/server/file_management_handlers.go excalidraw-be/cmd/server/main.go
git commit -m "feat(be): add SaveFileTabs endpoint for persisting tab data"
```

---

## Task 9: Manual Testing

**Step 1: Build backend**
```bash
cd excalidraw-be && make build
```

**Step 2: Build frontend**
```bash
cd excalidraw-fe && npm run build
```

**Step 3: Manual test flow**
1. Start app with `make dev`
2. As guest: create 2-3 files with sheets and draw elements
3. Register a new account
4. Verify: All local files are still visible
5. Verify: Files now show cloud icon
6. Refresh page → files should load from cloud with all data
7. Test: Login on different browser → same files appear

---

## Summary of Changes

| Component | Change | Purpose |
|-----------|--------|---------|
| `migrations/000008_file_tabs.*` | New table | Store per-tab canvas data |
| `database/file_tabs.go` | New file | Repository CRUD for tabs |
| `file_management_handlers.go` | 3 new endpoints | Migrate, list with tabs, save tabs |
| `main.go` | 2 new routes | Register migrate + save-tabs endpoints |
| `fileService.ts` | New methods | migrateFiles, saveFileTabs, updated types |
| `useWhiteboardStore.ts` | Merge logic | Preserve local data on login, migrate to cloud |

# Persistence Comparison

## 💾 Overview

Comparison between current project implementation and technical specification for persistence features including database, file storage, and file handling.

---

## ❌ Not Implemented Features

| Spec Requirement | Spec Details | Current Status | Priority |
| ---------------- | ------------ | -------------- | --------- |
| **PostgreSQL Database** | Rooms, room_elements, room_files tables | ❌ In-memory only (no database) | 🔴 High |
| **Room Table** | UUID id, key, name, created_at, updated_at | ❌ Not implemented | 🔴 High |
| **Room Elements Table** | room_id, element_id, version, data (JSONB), updated_at | ❌ Not implemented | 🔴 High |
| **Room Files Table** | room_id, file_id, s3_key, size, mime_type, created_at | ❌ Not implemented | 🔴 High |
| **Database Indexes** | Indexes on room_id for fast queries | ❌ Not implemented | 🟡 Medium |
| **Batch Element Save** | INSERT ... ON CONFLICT DO UPDATE | ❌ Not implemented | 🔴 High |
| **Throttled Persistence** | Save every 3 seconds | ❌ Not implemented | 🔴 High |
| **PostgreSQL Connection Pool** | Manage DB connections | ❌ Not implemented | 🔴 High |
| **Migrations** | Schema version management | ❌ Not implemented | 🟡 Medium |
| **S3/MinIO Storage** | File storage for images | ❌ Not implemented | 🔴 High |
| **File Upload Endpoint** | POST /api/files/upload | ❌ Not implemented | 🔴 High |
| **File Download Endpoint** | GET /api/files/{roomId}/{fileId} | ❌ Not implemented | 🔴 High |
| **Firebase Storage Alternative** | For file persistence | ❌ Not implemented | 🔴 High |
| **File Encoding for Upload** | Encode files with encryption | ❌ Not implemented | 🔴 High |
| **File Decryption** | Decrypt files on download | ❌ Not implemented | 🔴 High |
| **Scene Persistence to Firebase** | Save full scene to Firebase | ❌ Not implemented | 🔴 High |
| **Initial Scene Fetch** | Load scene from Firebase when joining room | ❌ Not implemented | 🔴 High |
| **Get Elements API** | GET /api/rooms/{roomId}/elements | ⚠️ Partial (via WebSocket only) | 🟡 Medium |
| **Create Room API** | POST /api/rooms | ⚠️ Partial (via WebSocket only) | 🟡 Medium |
| **Get Room API** | GET /api/rooms/{roomId} | ⚠️ Partial (via WebSocket only) | 🟡 Medium |

---

## 📊 Database Schema Comparison

### Spec - PostgreSQL Schema:
```sql
-- Rooms table
CREATE TABLE rooms (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key        VARCHAR(64) NOT NULL,
    name       VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Room elements (scene data)
CREATE TABLE room_elements (
    id         SERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    element_id VARCHAR(255) NOT NULL,
    version    INTEGER DEFAULT 1,
    data       JSONB NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, element_id)
);

CREATE INDEX idx_room_elements_room_id ON room_elements(room_id);

-- Room files (images)
CREATE TABLE room_files (
    id         SERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    file_id    VARCHAR(255) NOT NULL,
    s3_key     VARCHAR(512) NOT NULL,
    size       BIGINT,
    mime_type  VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, file_id)
);

CREATE INDEX idx_room_files_room_id ON room_files(room_id);
```

### Current - In-Memory Only:
```go
// Current: In-memory only (no database)
type Room struct {
    ID         string
    Elements   map[string]*Element  // Lost on restart
    Cursors    map[string]*Cursor
    SelectedIDs map[string][]string
    mutex      sync.RWMutex
}

// No persistence - all data lost on:
// - Server restart
// - Application crash
// - System reboot
```

---

## 🔍 Persistence Flow Comparison

### Spec - Persistence Flow:
```
1. User draws elements
2. Element changes collected in pending updates
3. 3-second throttle triggers
4. Batch INSERT ... ON CONFLICT to PostgreSQL
5. Room join → Fetch initial scene from PostgreSQL
6. Reconnect → Restore state from PostgreSQL
7. Server restart → All data preserved in database
```

### Current - Persistence Flow:
```
1. User draws elements
2. Element changes stored in-memory
3. No database writes
4. Page refresh → All data lost
5. Server restart → All rooms/elements lost
6. Application crash → All data lost
7. No recovery possible
```

---

## 📁 REST API Comparison

| Endpoint | Method | Spec | Current | Status |
| -------- | ------ | ---- | ------- | ------ |
| `/ws/:roomId` | WS | ✅ Specified | ✅ Implemented | ✅ Complete |
| `/api/rooms` | POST | ✅ Specified | ⚠️ Via WebSocket only | ⚠️ Partial |
| `/api/rooms/:id` | GET | ✅ Specified | ⚠️ Via WebSocket only | ⚠️ Partial |
| `/api/rooms/:id/elements` | GET | ✅ Specified | ⚠️ Via WebSocket only | ⚠️ Partial |
| `/api/rooms/:id/link` | GET | ✅ Specified | ✅ Implemented | ✅ Complete |
| `/api/files/upload` | POST | ✅ Specified | ❌ Not implemented | ❌ Missing |
| `/api/files/:roomId/:fileId` | GET | ✅ Specified | ❌ Not implemented | ❌ Missing |

---

## 🗄️ PostgreSQL Integration Gap

### Spec - PostgreSQL Client:
```go
type PostgresClient struct {
    db *sql.DB
}

func (p *PostgresClient) BatchSaveElements(roomID string, updates []ElementUpdate) error {
    tx, _ := p.db.Begin()
    stmt, _ := tx.Prepare(`
        INSERT INTO room_elements (room_id, element_id, data, version, updated_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (room_id, element_id)
        DO UPDATE SET data = $3, version = $4, updated_at = NOW()
    `)

    for _, u := range updates {
        stmt.Exec(roomID, u.ElementID, u.Data, u.Version)
    }
    return tx.Commit()
}

// Throttled persistence (every 3 seconds)
func (r *Room) StartPersistence(db *storage.Postgres, interval time.Duration) {
    ticker := time.NewTicker(interval)
    pending := make([]ElementUpdate, 0)

    go func() {
        for {
            select {
            case update := <-r.elementCh:
                pending = append(pending, update)
            case <-ticker.C:
                if len(pending) > 0 {
                    db.BatchSaveElements(r.ID, pending)
                    pending = pending[:0]
                }
            }
        }
    }()
}
```

### Current - No Database:
```go
// ❌ Not implemented - No database connection

// Current implementation only uses in-memory storage
type Room struct {
    Elements map[string]*Element
    mutex   sync.RWMutex
}

// No persistence layer
// No database connection pool
// No transaction management
// No query execution
// No error handling for database failures
```

---

## 📦 File Storage Gap

### Spec - S3/MinIO Storage:
```go
type S3Client struct {
    client *minio.Client
}

func (s *S3Client) UploadFile(ctx context.Context, roomID, fileID string, data []byte) (string, error) {
    key := fmt.Sprintf("rooms/%s/files/%s", roomID, fileID)
    _, err := s.client.PutObject(ctx, "excalidraw-files", key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
    return key, err
}

func (s *S3Client) DownloadFile(ctx context.Context, key string) ([]byte, error) {
    obj, err := s.client.GetObject(ctx, "excalidraw-files", key, minio.GetObjectOptions{})
    if err != nil {
        return nil, err
    }
    defer obj.Close()

    return io.ReadAll(obj)
}
```

### Current - No File Storage:
```go
// ❌ Not implemented - No file storage

// Files are only handled client-side
// No S3/MinIO integration
// No file upload endpoint
// No file download endpoint
// No file encryption
// No file tracking
```

---

## 🔄 File Handling Comparison

| File Operation | Spec Implementation | Current Implementation | Gap |
| -------------- | ------------------ | --------------------- | ---- |
| **Upload Files** | Encode → Encrypt → Upload to S3 | ❌ No server-side file handling | No file storage infrastructure |
| **File Storage** | S3/MinIO or Firebase Storage | ❌ No storage integration | No object storage configured |
| **File Encryption** | AES-GCM encryption before upload | ❌ No encryption | Files stored plaintext if implemented |
| **Download Files** | Fetch from S3 → Decrypt → Decode | ❌ No download endpoint | No file retrieval mechanism |
| **File References** | Tracked in room_files table | ❌ No file tracking | No database table for files |
| **Image Fetching** | Lazy load images when needed | ❌ No lazy loading | Images not loaded dynamically |
| **File Size Limit** | 2MB per file | ❌ No size validation | No upload size restrictions |
| **Max Bytes Total** | Configurable limit per room | ❌ No limit enforcement | No storage quota management |

---

## 📤 File Upload Workflow Gap

### Spec - File Upload Workflow:
```javascript
class FileManager {
    async saveFiles({ addedFiles }) {
        // Encode files for upload with encryption
        const encoded = await encodeFilesForUpload({
            files: addedFiles,
            encryptionKey: roomKey,
            maxBytes: FILE_UPLOAD_MAX_BYTES, // 2MB per file
        });

        // Upload to Firebase Storage
        const { savedFiles, erroredFiles } = await saveFilesToFirebase({
            prefix: `${FIREBASE_STORAGE_PREFIXES.collabFiles}/${roomId}`,
            files: encoded,
        });

        // Save file metadata to database
        for (const file of savedFiles) {
            await saveFileMetadata({
                roomId,
                fileId: file.id,
                s3Key: file.s3Key,
                size: file.data.size,
                mimeType: file.mimeType,
            });
        }

        return { savedFiles, erroredFiles };
    }
}
```

### Current - No File Upload:
```javascript
// ❌ Not implemented - No server-side file handling

// Files are only stored in browser memory
// No upload functionality
// No encryption before upload
// No file metadata storage
// No error handling for failed uploads
```

---

## 📥 File Download Workflow Gap

### Spec - File Download Workflow:
```javascript
async function fetchImageFilesFromFirebase(opts) {
    const unfetchedImages = opts.elements
        .filter(element =>
            isInitializedImageElement(element) &&
            !fileManager.isFileTracked(element.fileId) &&
            element.status === "saved"
        )
        .map(element => element.fileId);

    if (unfetchedImages.length === 0) {
        return null;
    }

    // Fetch files from Firebase
    const files = await fileManager.getFiles(unfetchedImages);

    // Decrypt files
    const decryptedFiles = await Promise.all(
        files.map(async (file) => ({
            ...file,
            data: await decryptFile(file.data, roomKey),
        }))
    );

    return { files: decryptedFiles };
}
```

### Current - No File Download:
```javascript
// ❌ Not implemented - No file download endpoint

// Images are not loaded from server
// No lazy loading
// No decryption
// No file caching
```

---

## 🔐 File Encryption Gap

### Spec - File Encryption:
```javascript
async function encodeFilesForUpload(opts) {
    const { files, encryptionKey, maxBytes } = opts;

    const encodedFiles = [];

    for (const [id, file] of files) {
        try {
            // Validate file size
            if (file.data.byteLength > maxBytes) {
                throw new Error(`File size exceeds limit`);
            }

            // Encrypt file data
            const encryptedData = await encryptData(encryptionKey, file.data);

            encodedFiles.push({
                id,
                mimeType: file.mimeType,
                dataURL: await createDataURL(encryptedData, file.mimeType),
            });
        } catch (error) {
            console.error(`Failed to encode file ${id}:`, error);
            encodedFiles.push({ id, mimeType: file.mimeType, error });
        }
    }

    return encodedFiles;
}
```

### Current - No Encryption:
```javascript
// ❌ Not implemented - No file encryption

// Files are not encrypted
// No encryption key management
// No data URL generation
// No error handling for encryption failures
```

---

## 📈 Persistence Completion

| Component | Progress |
| --------- | -------- |
| **Database Schema** | 0% ❌ |
| **PostgreSQL Integration** | 0% ❌ |
| **Element Persistence** | 0% ❌ |
| **File Storage** | 0% ❌ |
| **File Upload** | 0% ❌ |
| **File Download** | 0% ❌ |
| **File Encryption** | 0% ❌ |
| **REST API** | 30% ⚠️ |
| **Data Recovery** | 0% ❌ |

**Overall Persistence**: **~10% Complete** 💾

---

## 🚀 Implementation Roadmap

### Phase 11: Database Setup (High Priority 🔴)
1. **Install dependencies**: `lib/pq` PostgreSQL driver
2. **Database schema**: Create migration files for tables
3. **Connection pool**: Implement `sql.DB` connection management
4. **Migration system**: Add schema version tracking
5. **Initial migration**: Run schema creation scripts

### Phase 12: Element Persistence (High Priority 🔴)
1. **Room CRUD operations**: Create room in database on creation
2. **Batch element save**: Implement `BatchSaveElements()` with UPSERT
3. **Throttled persistence**: Add 3-second timer for batching
4. **Element fetch**: Implement `GetElements()` by room_id
5. **Initial scene load**: Fetch from DB on room join
6. **Version tracking**: Track element versions for conflict resolution

### Phase 13: File Storage (High Priority 🔴)
1. **S3/MinIO setup**: Configure object storage
2. **S3 client**: Implement `S3Client` with upload/download
3. **File upload endpoint**: POST `/api/files/upload`
4. **File download endpoint**: GET `/api/files/{roomId}/{fileId}`
5. **File metadata**: Save to `room_files` table
6. **File validation**: Enforce 2MB per file limit

### Phase 14: File Encryption (High Priority 🔴)
1. **File encoding**: Implement `encodeFilesForUpload()`
2. **File encryption**: Use AES-GCM for file data
3. **Data URL generation**: Convert encrypted data to data URLs
4. **File decryption**: Implement `decryptFile()` on download
5. **Error handling**: Handle encryption/decryption failures
6. **Size validation**: Enforce max bytes per file

### Phase 15: REST API Completion (Medium Priority 🟡)
1. **Create room API**: POST `/api/rooms` with JSON response
2. **Get room API**: GET `/api/rooms/:id` with room details
3. **Get elements API**: GET `/api/rooms/:id/elements` with JSON elements
4. **Update room API**: PUT `/api/rooms/:id` for room metadata
5. **Delete room API**: DELETE `/api/rooms/:id` with CASCADE
6. **API documentation**: Add OpenAPI/Swagger specs

### Phase 16: Advanced Features (Low Priority 🟢)
1. **Database indexes**: Optimize query performance
2. **Connection pooling**: Tune pool settings
3. **Query optimization**: Add EXPLAIN ANALYZE for slow queries
4. **Backup strategy**: Implement automated backups
5. **Replication**: Set up read replicas (optional)

---

## 📝 Implementation Notes

### Critical Considerations:
1. **Database connection pooling**: Use `sql.DB` with proper connection limits
2. **Transaction management**: Use transactions for multi-operation commits
3. **Error handling**: Handle connection failures gracefully
4. **Indexing**: Add indexes on `room_id` for all tables
5. **Cascading deletes**: Ensure proper cleanup when rooms are deleted
6. **File size limits**: Enforce 2MB per file limit to prevent abuse
7. **Encryption key management**: Securely store encryption keys
8. **Backup strategy**: Implement regular database backups

### Performance Optimizations:
1. **Batch inserts**: Use `INSERT ... ON CONFLICT` for upserts
2. **Throttled persistence**: 3-second timer to reduce DB load
3. **Connection reuse**: Reuse database connections
4. **Prepared statements**: Use prepared statements for repeated queries
5. **Lazy loading**: Load files only when needed
6. **Caching**: Cache frequently accessed data

### Security Considerations:
1. **File encryption**: All files must be encrypted before upload
2. **Database credentials**: Never hardcode credentials
3. **SQL injection**: Use prepared statements to prevent injection
4. **File validation**: Validate file types and sizes
5. **Access control**: Restrict access to room files
6. **Audit logging**: Log all database operations

---

## 🔍 Current Limitations

1. **Data Loss**: All data lost on server restart
2. **No Recovery**: No way to recover lost data
3. **No File Storage**: Files only stored in browser memory
4. **No Collaboration History**: No way to track changes over time
5. **No Analytics**: Cannot track room usage statistics
6. **No Scaling**: Limited to single instance due to in-memory storage
7. **No Backup**: No automated backup mechanism

---

## 🎯 Success Criteria

Persistence will be complete when:
- ✅ All room data persists across server restarts
- ✅ Files can be uploaded and downloaded via API
- ✅ Files are encrypted before storage
- ✅ Database has proper indexing for performance
- ✅ REST API provides full CRUD operations
- ✅ Backup strategy is in place
- ✅ Error handling is robust and user-friendly
- ✅ Performance is acceptable (sub-100ms query times)

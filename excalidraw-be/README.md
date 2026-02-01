# Excalidraw Backend

Real-time collaboration backend for Excalidraw whiteboard application, built with Go.

## Technology Stack

- **HTTP Framework**: [go-chi/chi](https://github.com/go-chi/chi) - Lightweight, idiomatic router
- **WebSocket**: [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket library
- **Logging**: [zap](https://github.com/uber-go/zap) - Fast, structured logging
- **Configuration**: [viper](https://github.com/spf13/viper) - Configuration management
- **UUID**: [google/uuid](https://github.com/google/uuid) - UUID generation

## Features

### Phase 1 & Phase 2 (MVP Core)
- ✅ HTTP server with health check endpoint
- ✅ WebSocket server for real-time communication
- ✅ In-memory room management
- ✅ Thread-safe room operations with mutexes
- ✅ Room join/leave functionality
- ✅ Automatic cleanup of inactive rooms
- ✅ Structured logging with zap
- ✅ Graceful shutdown handling

## Project Structure

```
excalidraw-be/
├── cmd/
│   └── server/
│       └── main.go           # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── middleware/
│   │   └── logging.go        # HTTP middleware
│   ├── room/
│   │   ├── room.go           # Room data structures
│   │   └── manager.go        # Room management
│   └── websocket/
│       ├── handler.go        # WebSocket connection handler
│       ├── types.go          # WebSocket message types
│       └── upgrader.go       # WebSocket upgrader
├── pkg/                       # Public packages
├── config.yaml                # Configuration file
├── .env.example              # Environment variables template
├── Makefile                  # Build and run commands
├── go.mod
└── go.sum
```

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Make (optional, for convenience commands)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd excalidraw-be
```

2. Install dependencies:
```bash
make deps
```

3. Create environment file:
```bash
cp .env.example .env
```

4. Run the server:
```bash
make run
```

Or use go directly:
```bash
go run ./cmd/server
```

The server will start on port 8080 by default.

### API Endpoints

#### Health Check
```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2026-01-31T12:00:00Z"
}
```

#### WebSocket Connection
```bash
GET /ws
```

### WebSocket Events

#### Client → Server

**join_room**
```json
{
  "type": "join_room",
  "payload": {
    "roomId": "abc123",
    "username": "John Doe"
  }
}
```

**leave_room**
```json
{
  "type": "leave_room",
  "payload": {
    "roomId": "abc123"
  }
}
```

**update_elements**
```json
{
  "type": "update_elements",
  "payload": {
    "roomId": "abc123",
    "changes": {
      "added": [
        {
          "id": "elem1",
          "type": "rectangle",
          "x": 100,
          "y": 100,
          "width": 200,
          "height": 100
        }
      ],
      "updated": [],
      "deleted": []
    }
  }
}
```

**cursor_move**
```json
{
  "type": "cursor_move",
  "payload": {
    "roomId": "abc123",
    "position": {
      "x": 250.5,
      "y": 150.3
    }
  }
}
```

#### Server → Client

**room_state** (sent on join)
```json
{
  "type": "room_state",
  "payload": {
    "elements": [...],
    "participants": [...]
  }
}
```

**user_joined**
```json
{
  "type": "user_joined",
  "payload": {
    "userId": "user-123",
    "username": "John Doe",
    "color": "#FF6B6B"
  }
}
```

**user_left**
```json
{
  "type": "user_left",
  "payload": {
    "userId": "user-123"
  }
}
```

**elements_updated**
```json
{
  "type": "elements_updated",
  "payload": {
    "userId": "user-456",
    "changes": {
      "added": [...],
      "updated": [...],
      "deleted": [...]
    }
  }
}
```

**cursor_updated**
```json
{
  "type": "cursor_updated",
  "payload": {
    "userId": "user-456",
    "username": "Jane Doe",
    "color": "#4ECDC4",
    "position": {
      "x": 300.2,
      "y": 200.8
    }
  }
}
```

**error**
```json
{
  "type": "error",
  "payload": {
    "message": "Room is full",
    "code": "room_full"
  }
}
```

### Configuration

Configuration can be provided via:
1. Environment variables (prefixed with `EXCALIDRAW_`)
2. `config.yaml` file

Example `config.yaml`:
```yaml
server:
  port: "8080"
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 60s

websocket:
  read_buffer_size: 1024
  write_buffer_size: 1024
  ping_period: 54s
  pong_wait: 60s

room:
  capacity: 50
  inactivity_timeout: 1h
  cleanup_interval: 10m

log:
  level: info
  format: json
```

### Development

#### Run tests
```bash
make test
```

#### Run tests with coverage
```bash
make test-coverage
```

#### Format code
```bash
make fmt
```

#### Run linter
```bash
make lint
```

#### Build binary
```bash
make build
```

The binary will be created in `bin/server`.

#### Development mode with hot reload
```bash
make dev
```

Requires [air](https://github.com/cosmtrek/air) to be installed:
```bash
make install-tools
```

## Room Management

- Rooms are created automatically when the first user joins
- Rooms are stored in memory (data is lost on server restart)
- Inactive rooms (no participants for 1 hour) are automatically cleaned up
- Maximum 50 participants per room (configurable)

## Concurrency Safety

- All room operations are thread-safe using `sync.RWMutex`
- WebSocket connections use buffered channels for message passing
- Room manager uses goroutines for background cleanup

## Logging

Structured logging with zap supports two formats:
- `json` - Production-friendly JSON logging
- `console` - Human-readable console logging

Log levels: `debug`, `info`, `warn`, `error`

## Testing

The project includes comprehensive tests for:
- Room operations
- WebSocket message handling
- Connection lifecycle

Run tests with:
```bash
make test
```

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

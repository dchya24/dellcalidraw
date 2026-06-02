-- Phase 11: per-room AES-256-GCM encryption key for WebSocket message encryption.
-- Stored as base64 (44 chars). Existing rooms get NULL and lazily generate
-- a key on first load.
ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS encryption_key VARCHAR(64);

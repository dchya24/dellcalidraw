-- Phase 11: Advanced Room Features - Room Permissions

-- Add room settings columns
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS is_public BOOLEAN DEFAULT TRUE;
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS allow_anonymous BOOLEAN DEFAULT TRUE;

-- Room members table for persistent role assignments
CREATE TABLE IF NOT EXISTS room_members (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(20) NOT NULL DEFAULT 'editor', -- owner, editor, viewer
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_room_members_room_id ON room_members(room_id);
CREATE INDEX IF NOT EXISTS idx_room_members_user_id ON room_members(user_id);

-- Room invitations for pending invites
CREATE TABLE IF NOT EXISTS room_invitations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    email      VARCHAR(255),
    role       VARCHAR(20) NOT NULL DEFAULT 'editor',
    token      VARCHAR(255) NOT NULL UNIQUE,
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at    TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_room_invitations_room_id ON room_invitations(room_id);
CREATE INDEX IF NOT EXISTS idx_room_invitations_token ON room_invitations(token);
CREATE INDEX IF NOT EXISTS idx_room_invitations_email ON room_invitations(email);

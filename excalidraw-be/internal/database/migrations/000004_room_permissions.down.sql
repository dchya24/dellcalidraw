-- Rollback Phase 11: Advanced Room Features

DROP TABLE IF EXISTS room_invitations;
DROP TABLE IF EXISTS room_members;

ALTER TABLE rooms DROP COLUMN IF EXISTS allow_anonymous;
ALTER TABLE rooms DROP COLUMN IF EXISTS is_public;
ALTER TABLE rooms DROP COLUMN IF EXISTS password_hash;
ALTER TABLE rooms DROP COLUMN IF EXISTS owner_id;

-- 002_chirps.sql

-- +goose Up
CREATE TABLE IF NOT EXISTS chirps (
    id UUID PRIMARY KEY DEFAUlT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    body TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS chirps;
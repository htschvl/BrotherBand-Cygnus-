-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

CREATE TABLE users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    username      CITEXT       NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,
    birthdate     DATE         NOT NULL,
    secret        TEXT         NOT NULL,
    status        TEXT         NOT NULL,
    favorites     TEXT[]       NOT NULL,
    avatar_key    TEXT,
    registered_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT users_favorites_size CHECK (cardinality(favorites) = 5)
);

CREATE TABLE brotherband_requests (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT brotherband_requests_distinct_users CHECK (requester_id <> recipient_id),
    CONSTRAINT brotherband_requests_unique_pair UNIQUE (requester_id, recipient_id)
);
CREATE INDEX brotherband_requests_recipient ON brotherband_requests (recipient_id, created_at DESC);
CREATE INDEX brotherband_requests_requester ON brotherband_requests (requester_id, created_at DESC);

-- The brotherhood is symmetric. We store it as a single canonical row
-- per pair, with user_low_id < user_high_id, so the relationship has
-- exactly one source of truth and one row to delete on cut.
CREATE TABLE brotherhoods (
    user_low_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_high_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    became_brothers_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_low_id, user_high_id),
    CONSTRAINT brotherhoods_canonical_order CHECK (user_low_id < user_high_id)
);
CREATE INDEX brotherhoods_high_lookup ON brotherhoods (user_high_id);

CREATE TABLE conversations (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE conversation_participants (
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_read_at    TIMESTAMPTZ,
    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX conversation_participants_user ON conversation_participants (user_id);

CREATE TABLE messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       UUID        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    body            TEXT        NOT NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at       TIMESTAMPTZ
);
CREATE INDEX messages_conversation_created
    ON messages (conversation_id, created_at DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversation_participants;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS brotherhoods;
DROP TABLE IF EXISTS brotherband_requests;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd

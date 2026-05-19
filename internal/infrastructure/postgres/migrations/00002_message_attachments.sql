-- +goose Up
-- +goose StatementBegin
CREATE TABLE message_attachments (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id   UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    media_key    TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL CHECK (size_bytes > 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX message_attachments_message ON message_attachments (message_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS message_attachments;
-- +goose StatementEnd

CREATE TABLE queue_items (
    id               text PRIMARY KEY,
    seq              bigserial NOT NULL UNIQUE,
    session_id       text NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    video_id         text NOT NULL,
    title            text NOT NULL DEFAULT '',
    channel          text NOT NULL DEFAULT '',
    thumbnail_url    text NOT NULL DEFAULT '',
    duration_seconds integer NOT NULL DEFAULT 0,
    is_backup        boolean NOT NULL DEFAULT false,
    requested_by     text NOT NULL DEFAULT '',
    status           text NOT NULL DEFAULT 'queued'
                     CHECK (status IN ('queued', 'playing', 'played', 'skipped', 'removed')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz NULL,
    finished_at      timestamptz NULL
);

CREATE INDEX queue_items_session_status_seq_idx ON queue_items (session_id, status, seq);

ALTER TABLE queue_items ENABLE ROW LEVEL SECURITY;

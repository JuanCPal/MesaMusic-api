CREATE TABLE sessions (
    id         text PRIMARY KEY,
    type       text NOT NULL DEFAULT 'meeting' CHECK (type IN ('meeting', 'business')),
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended')),
    created_at timestamptz NOT NULL DEFAULT now(),
    ended_at   timestamptz NULL
);

ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;

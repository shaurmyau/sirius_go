CREATE TABLE IF NOT EXISTS public.image (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    name        VARCHAR(255) NOT NULL,
    mime_type   VARCHAR(128) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    size        BIGINT NOT NULL DEFAULT 0,
    size_large  BIGINT NOT NULL DEFAULT 0,
    size_medium BIGINT NOT NULL DEFAULT 0,
    size_small  BIGINT NOT NULL DEFAULT 0,
    status      VARCHAR(16) NOT NULL DEFAULT 'upload'
);

-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email CITEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    password_hash TEXT,
    role TEXT NOT NULL CHECK (role IN ('admin', 'verifier')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by BIGINT REFERENCES users(id)
);
CREATE TABLE invitations (
    id BIGSERIAL PRIMARY KEY,
    email CITEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'verifier')),
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL REFERENCES users(id)
);
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON sessions (expires_at);
CREATE TABLE login_attempts (
    key_hash TEXT PRIMARY KEY,
    failures INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE entries (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    headword TEXT NOT NULL CHECK (char_length(headword) BETWEEN 1 AND 60),
    normalized TEXT NOT NULL,
    origin TEXT NOT NULL DEFAULT 'belum_diketahui'
        CHECK (origin IN ('asli','serapan_id','serapan_en','serapan_zh','serapan_melayu','belum_diketahui')),
    relation TEXT NOT NULL DEFAULT 'belum_dinilai'
        CHECK (relation IN ('sama','mirip','berbeda','tidak_ada_padanan','belum_dinilai')),
    malay_form TEXT,
    visibility TEXT NOT NULL DEFAULT 'published' CHECK (visibility IN ('published','hidden')),
    is_sensitive BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX entries_norm_trgm ON entries USING gin (normalized gin_trgm_ops);
CREATE INDEX entries_public_alphabet ON entries (normalized text_pattern_ops, id)
    WHERE visibility = 'published' AND NOT is_sensitive;
CREATE TABLE submissions (
    id BIGSERIAL PRIMARY KEY,
    headword TEXT NOT NULL CHECK (char_length(headword) BETWEEN 1 AND 60),
    gloss_id TEXT NOT NULL CHECK (char_length(gloss_id) BETWEEN 1 AND 1000),
    explanation TEXT NOT NULL DEFAULT '' CHECK (char_length(explanation) <= 1000),
    pos TEXT NOT NULL DEFAULT '',
    example_lintang TEXT NOT NULL DEFAULT '',
    example_id TEXT NOT NULL DEFAULT '',
    submitter_name TEXT NOT NULL DEFAULT '',
    submitter_contact TEXT NOT NULL DEFAULT '',
    credit_consent BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','needs_fix')),
    ip_hash TEXT,
    fingerprint_hash TEXT,
    submitter_session_hash TEXT,
    submitter_user_id BIGINT REFERENCES users(id),
    reviewed_by BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    review_note TEXT NOT NULL DEFAULT '',
    resulting_entry_id BIGINT REFERENCES entries(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON submissions (status, created_at);
CREATE INDEX ON submissions (headword, gloss_id, created_at) WHERE status = 'pending';
CREATE TABLE spellings (
    id BIGSERIAL PRIMARY KEY,
    entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    form TEXT NOT NULL,
    normalized TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (entry_id, form)
);
CREATE UNIQUE INDEX spelling_primary ON spellings (entry_id) WHERE is_primary;
CREATE INDEX spellings_norm_trgm ON spellings USING gin (normalized gin_trgm_ops);
CREATE TABLE senses (
    id BIGSERIAL PRIMARY KEY,
    entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 1,
    pos TEXT NOT NULL DEFAULT '',
    gloss_id TEXT NOT NULL CHECK (char_length(gloss_id) BETWEEN 1 AND 1000),
    explanation TEXT NOT NULL DEFAULT '',
    search_vec TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple'::regconfig, gloss_id || ' ' || explanation)
    ) STORED
);
CREATE INDEX senses_fts ON senses USING gin (search_vec);
CREATE INDEX senses_gloss_trgm ON senses USING gin (gloss_id gin_trgm_ops);
CREATE INDEX senses_gloss_exact ON senses (lower(gloss_id));
CREATE INDEX ON senses (entry_id, position, id);
CREATE TABLE examples (
    id BIGSERIAL PRIMARY KEY,
    sense_id BIGINT NOT NULL REFERENCES senses(id) ON DELETE CASCADE,
    text_lintang TEXT NOT NULL,
    text_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON examples (sense_id);
CREATE TABLE blocklist (
    ip_hash TEXT PRIMARY KEY,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE rate_limits (
    key_hash TEXT PRIMARY KEY,
    hour_start TIMESTAMPTZ NOT NULL,
    hour_count INTEGER NOT NULL,
    day_start TIMESTAMPTZ NOT NULL,
    day_count INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Enforce BR-4 at transaction commit, after the entry and its children are inserted.
-- +goose StatementBegin
CREATE FUNCTION check_entry_complete() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target_id BIGINT;
BEGIN
    IF TG_TABLE_NAME = 'entries' THEN
        target_id := NEW.id;
    ELSIF TG_OP = 'DELETE' THEN
        target_id := OLD.entry_id;
    ELSE
        target_id := NEW.entry_id;
    END IF;
    IF EXISTS (SELECT 1 FROM entries WHERE id = target_id) AND (
        NOT EXISTS (SELECT 1 FROM spellings WHERE entry_id = target_id AND is_primary)
        OR NOT EXISTS (SELECT 1 FROM senses WHERE entry_id = target_id)
    ) THEN
        RAISE EXCEPTION 'entry requires primary spelling and sense';
    END IF;
    IF TG_TABLE_NAME <> 'entries' AND TG_OP = 'UPDATE' THEN
        IF OLD.entry_id <> NEW.entry_id
           AND EXISTS (SELECT 1 FROM entries WHERE id = OLD.entry_id) AND (
            NOT EXISTS (SELECT 1 FROM spellings WHERE entry_id = OLD.entry_id AND is_primary)
            OR NOT EXISTS (SELECT 1 FROM senses WHERE entry_id = OLD.entry_id)
        ) THEN
            RAISE EXCEPTION 'previous entry requires primary spelling and sense';
        END IF;
    END IF;
    RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER entry_complete AFTER INSERT OR UPDATE ON entries
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_entry_complete();
CREATE CONSTRAINT TRIGGER spelling_complete AFTER INSERT OR UPDATE OR DELETE ON spellings
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_entry_complete();
CREATE CONSTRAINT TRIGGER sense_complete AFTER INSERT OR UPDATE OR DELETE ON senses
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_entry_complete();

-- +goose Down
DROP TABLE rate_limits, blocklist, examples, senses, spellings, submissions, entries,
    login_attempts, sessions, invitations, users CASCADE;
DROP FUNCTION check_entry_complete();

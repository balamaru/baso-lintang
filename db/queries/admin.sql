-- name: Queue :many
SELECT * FROM submissions
WHERE status = sqlc.arg(status)::text
AND (sqlc.arg(query)::text = '' OR strpos(lower(headword), lower(sqlc.arg(query)::text)) > 0
OR strpos(lower(gloss_id), lower(sqlc.arg(query)::text)) > 0)
ORDER BY
CASE WHEN sqlc.arg(newest)::boolean THEN created_at END DESC,
CASE WHEN NOT sqlc.arg(newest)::boolean THEN created_at END ASC, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: GetSubmission :one
SELECT * FROM submissions WHERE id = $1;

-- name: LockSubmission :one
SELECT * FROM submissions WHERE id = $1 FOR UPDATE;

-- name: CreateEntry :one
INSERT INTO entries (slug, headword, normalized) VALUES ($1,$2,$3) RETURNING id;

-- name: LockEntry :one
SELECT id FROM entries WHERE id = $1 FOR UPDATE;

-- name: AddSpelling :exec
INSERT INTO spellings (entry_id, form, normalized, is_primary) VALUES ($1,$2,$3,$4)
ON CONFLICT (entry_id, form) DO NOTHING;

-- name: AddSense :one
INSERT INTO senses (entry_id, pos, gloss_id, explanation) VALUES ($1,$2,$3,$4) RETURNING id;

-- name: AddExample :exec
INSERT INTO examples (sense_id, text_lintang, text_id) VALUES ($1,$2,$3);

-- name: ReviewSubmission :exec
UPDATE submissions SET status=$2, reviewed_by=$3, reviewed_at=now(), review_note=$4,
 resulting_entry_id=$5, headword=$6, gloss_id=$7, explanation=$8, pos=$9,
 example_lintang=$10, example_id=$11 WHERE id=$1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: LockUser :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, role, created_by) VALUES ($1,$2,$3,$4,$5) RETURNING id;

-- name: ListUsers :many
SELECT id, email, name, role, is_active FROM users ORDER BY id LIMIT 20 OFFSET $1;

-- name: UpdateUser :exec
UPDATE users SET role = $2, is_active = $3 WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, now() + interval '7 days');

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: SessionUser :one
SELECT u.* FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.id = $1 AND s.expires_at > now() AND u.is_active;

-- name: TouchSession :exec
UPDATE sessions SET expires_at = now() + interval '7 days' WHERE id = $1 AND expires_at > now();

-- name: CreateInvitation :exec
INSERT INTO invitations (email, role, token_hash, expires_at, created_by)
VALUES ($1,$2,$3,now() + interval '72 hours',$4);

-- name: ConsumeInvitation :one
UPDATE invitations SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING *;

-- name: InitLoginAttempt :exec
INSERT INTO login_attempts (key_hash) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: LockLoginAttempt :one
SELECT failures, coalesce(locked_until > now(), false)::boolean AS locked
FROM login_attempts WHERE key_hash = $1 FOR UPDATE;

-- name: FailLogin :exec
UPDATE login_attempts SET
 failures = CASE WHEN locked_until <= now() THEN 1 ELSE failures + 1 END,
 locked_until = CASE WHEN locked_until <= now() THEN NULL
 WHEN failures + 1 >= 5 THEN now() + interval '15 minutes' ELSE NULL END,
 updated_at = now() WHERE key_hash = $1;

-- name: ClearLogin :exec
DELETE FROM login_attempts WHERE key_hash = $1;

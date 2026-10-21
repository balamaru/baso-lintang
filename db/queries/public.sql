-- name: SearchEntries :many
WITH matches AS (
    SELECT s.entry_id AS id, CASE WHEN s.normalized = sqlc.arg(query)::text THEN 1 ELSE 0 END AS exact,
           similarity(s.normalized, sqlc.arg(query)::text) AS score
    FROM spellings s
    WHERE sqlc.arg(direction)::text IN ('lintang','semua')
      AND (s.normalized = sqlc.arg(query)::text OR
           (s.normalized % sqlc.arg(query)::text AND similarity(s.normalized, sqlc.arg(query)::text) > 0.3))
    UNION ALL
    SELECT s.entry_id AS id, CASE WHEN lower(s.gloss_id) = sqlc.arg(query)::text THEN 1 ELSE 0 END AS exact,
           greatest(similarity(s.gloss_id, sqlc.arg(query)::text),
               ts_rank(s.search_vec, plainto_tsquery('simple', sqlc.arg(query)::text))) AS score
    FROM senses s
    WHERE sqlc.arg(direction)::text IN ('indonesia','semua')
      AND (lower(s.gloss_id) = sqlc.arg(query)::text OR s.gloss_id % sqlc.arg(query)::text
           OR s.search_vec @@ plainto_tsquery('simple', sqlc.arg(query)::text))
)
SELECT e.id, e.slug, e.headword, (SELECT gloss_id FROM senses WHERE entry_id = e.id ORDER BY position, id LIMIT 1)::text AS gloss
FROM entries e JOIN matches m ON e.id = m.id
WHERE e.visibility = 'published' AND NOT e.is_sensitive
GROUP BY e.id ORDER BY max(m.exact) DESC, max(m.score) DESC, e.headword, e.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListEntries :many
SELECT e.id, e.slug, e.headword, (SELECT gloss_id FROM senses WHERE entry_id = e.id ORDER BY position, id LIMIT 1)::text AS gloss
FROM entries e
WHERE e.visibility = 'published' AND NOT e.is_sensitive
AND starts_with(e.normalized, sqlc.arg(prefix)::text)
ORDER BY e.normalized, e.id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: PublicEntry :one
SELECT * FROM entries WHERE slug = $1 AND visibility = 'published' AND NOT is_sensitive;

-- name: PublicSpellings :many
SELECT s.* FROM spellings s JOIN entries e ON e.id = s.entry_id
WHERE e.id = $1 AND e.visibility = 'published' AND NOT e.is_sensitive ORDER BY s.is_primary DESC, s.id;

-- name: PublicSenses :many
SELECT s.id, s.entry_id, s.position, s.pos, s.gloss_id, s.explanation
FROM senses s JOIN entries e ON e.id = s.entry_id
WHERE e.id = $1 AND e.visibility = 'published' AND NOT e.is_sensitive ORDER BY s.position, s.id;

-- name: PublicExamples :many
SELECT x.* FROM examples x JOIN senses s ON s.id = x.sense_id JOIN entries e ON e.id = s.entry_id
WHERE e.id = $1 AND e.visibility = 'published' AND NOT e.is_sensitive ORDER BY x.id;

-- name: PublicCredits :many
SELECT DISTINCT s.submitter_name FROM submissions s JOIN entries e ON e.id = s.resulting_entry_id
WHERE e.id = $1 AND e.visibility = 'published' AND NOT e.is_sensitive
AND s.status = 'approved' AND s.credit_consent AND s.submitter_name <> '';

-- name: IsBlocked :one
SELECT EXISTS(SELECT 1 FROM blocklist WHERE ip_hash = $1 AND created_at > now() - interval '90 days');

-- name: ConsumeRate :one
INSERT INTO rate_limits (key_hash, hour_start, hour_count, day_start, day_count)
VALUES ($1, now(), 1, now(), 1)
ON CONFLICT (key_hash) DO UPDATE SET
 hour_count = CASE WHEN rate_limits.hour_start <= now() - interval '1 hour' THEN 1 ELSE rate_limits.hour_count + 1 END,
 hour_start = CASE WHEN rate_limits.hour_start <= now() - interval '1 hour' THEN now() ELSE rate_limits.hour_start END,
 day_count = CASE WHEN rate_limits.day_start <= now() - interval '1 day' THEN 1 ELSE rate_limits.day_count + 1 END,
 day_start = CASE WHEN rate_limits.day_start <= now() - interval '1 day' THEN now() ELSE rate_limits.day_start END,
 updated_at = now()
RETURNING hour_count, day_count;

-- name: SubmissionLock :exec
SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0));

-- name: PendingDuplicate :one
SELECT EXISTS(SELECT 1 FROM submissions WHERE headword = $1 AND gloss_id = $2
AND status = 'pending' AND created_at > now() - interval '24 hours');

-- name: CreateSubmission :one
INSERT INTO submissions (headword, gloss_id, explanation, pos, example_lintang, example_id,
 submitter_name, submitter_contact, credit_consent, ip_hash, fingerprint_hash, submitter_session_hash, submitter_user_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id;

-- name: PurgeSubmissionHashes :exec
UPDATE submissions SET ip_hash = NULL, fingerprint_hash = NULL, submitter_session_hash = NULL
WHERE created_at <= now() - interval '90 days'
AND (ip_hash IS NOT NULL OR fingerprint_hash IS NOT NULL OR submitter_session_hash IS NOT NULL);

-- name: PurgeRateLimits :exec
DELETE FROM rate_limits WHERE updated_at <= now() - interval '1 day';

-- name: PurgeBlocklist :exec
DELETE FROM blocklist WHERE created_at <= now() - interval '90 days';

-- name: PurgeSessions :exec
DELETE FROM sessions WHERE expires_at <= now();

-- name: PurgeLoginAttempts :exec
DELETE FROM login_attempts WHERE updated_at <= now() - interval '1 day';

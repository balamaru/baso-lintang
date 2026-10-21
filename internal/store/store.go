package store

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"

	"baso-lintang/internal/auth"
	"baso-lintang/internal/domain"
	"baso-lintang/internal/store/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool       *pgxpool.Pool
	Q          *dbgen.Queries
	Normalizer domain.Normalizer
}

func Open(ctx context.Context, url string, n domain.Normalizer) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("konfigurasi database tidak valid")
	}
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("database tidak tersedia")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("database tidak tersedia")
	}
	return &Store{Pool: pool, Q: dbgen.New(pool), Normalizer: n}, nil
}

func text(v string) pgtype.Text  { return pgtype.Text{String: v, Valid: v != ""} }
func number(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: v != 0} }

func (s *Store) transaction(ctx context.Context, fn func(*dbgen.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err = fn(s.Q.WithTx(tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Store) Blocked(ctx context.Context, key string) (bool, error) {
	return s.Q.IsBlocked(ctx, key)
}
func (s *Store) Allow(ctx context.Context, key string, hour, day int) (bool, error) {
	r, err := s.Q.ConsumeRate(ctx, key)
	if err != nil {
		return false, fmt.Errorf("rate limit: %w", err)
	}
	return int(r.HourCount) <= hour && int(r.DayCount) <= day, nil
}

func (s *Store) SaveSubmission(ctx context.Context, in domain.Submission) (bool, error) {
	duplicate := false
	err := s.transaction(ctx, func(q *dbgen.Queries) error {
		if err := q.SubmissionLock(ctx, auth.TokenHash(in.Headword+"\x00"+in.Gloss)); err != nil {
			return err
		}
		found, err := q.PendingDuplicate(ctx, dbgen.PendingDuplicateParams{Headword: in.Headword, GlossID: in.Gloss})
		if err != nil {
			return err
		}
		if found {
			duplicate = true
			return nil
		}
		_, err = q.CreateSubmission(ctx, dbgen.CreateSubmissionParams{
			Headword: in.Headword, GlossID: in.Gloss, Explanation: in.Explanation, Pos: in.POS,
			ExampleLintang: in.ExampleLintang, ExampleID: in.ExampleID, SubmitterName: in.Name,
			SubmitterContact: in.Contact, CreditConsent: in.Consent, IpHash: text(in.IPHash),
			FingerprintHash: text(in.FingerprintHash), SubmitterSessionHash: text(in.SessionHash), SubmitterUserID: number(in.UserID),
		})
		return err
	})
	return duplicate, err
}

func (s *Store) Review(ctx context.Context, r domain.Review) error {
	if err := domain.ValidateReview(r); err != nil {
		return err
	}
	return s.transaction(ctx, func(q *dbgen.Queries) error {
		reviewer, err := q.LockUser(ctx, r.ReviewerID)
		if err != nil {
			return err
		}
		if !reviewer.IsActive || (reviewer.Role != "admin" && reviewer.Role != "verifier") {
			return domain.ErrForbidden
		}
		sub, err := q.LockSubmission(ctx, r.SubmissionID)
		if err != nil {
			return err
		}
		if (sub.Status != "pending" && !(sub.Status == "needs_fix" && r.Action == "resubmit")) || (sub.Status == "pending" && r.Action == "resubmit") {
			return domain.ErrConflict
		}
		if err := domain.CanReview(sub.SubmitterUserID.Int64, sub.SubmitterSessionHash.String, r); err != nil {
			return err
		}
		entryID := int64(0)
		status := r.Action
		content := r.Content
		if r.Action == "approve" || r.Action == "merge" {
			status = "approved"
			if r.Action == "approve" {
				norm := s.Normalizer.Normalize(content.Headword)
				if norm == "" {
					return domain.ErrInvalid
				}
				entryID, err = q.CreateEntry(ctx, dbgen.CreateEntryParams{
					Slug: strings.ReplaceAll(norm, " ", "-") + "-" + strconv.FormatInt(sub.ID, 10), Headword: content.Headword, Normalized: norm})
			} else {
				entryID, err = q.LockEntry(ctx, r.TargetEntryID)
			}
			if err != nil {
				return err
			}
			for i, word := range append([]string{content.Headword}, r.Variants...) {
				if err = q.AddSpelling(ctx, dbgen.AddSpellingParams{EntryID: entryID, Form: word,
					Normalized: s.Normalizer.Normalize(word), IsPrimary: r.Action == "approve" && i == 0}); err != nil {
					return err
				}
			}
			senseID, err := q.AddSense(ctx, dbgen.AddSenseParams{EntryID: entryID, Pos: content.POS, GlossID: content.Gloss, Explanation: content.Explanation})
			if err != nil {
				return err
			}
			if content.ExampleLintang != "" {
				if err = q.AddExample(ctx, dbgen.AddExampleParams{SenseID: senseID, TextLintang: content.ExampleLintang, TextID: content.ExampleID}); err != nil {
					return err
				}
			}
		} else if r.Action == "resubmit" {
			status = "pending"
		} else {
			if status == "reject" {
				status = "rejected"
			}
			content = domain.Submission{Headword: sub.Headword, Gloss: sub.GlossID, Explanation: sub.Explanation, POS: sub.Pos, ExampleLintang: sub.ExampleLintang, ExampleID: sub.ExampleID}
		}
		return q.ReviewSubmission(ctx, dbgen.ReviewSubmissionParams{ID: sub.ID, Status: status, ReviewedBy: number(r.ReviewerID),
			ReviewNote: r.Note, ResultingEntryID: number(entryID), Headword: content.Headword, GlossID: content.Gloss,
			Explanation: content.Explanation, Pos: content.POS, ExampleLintang: content.ExampleLintang, ExampleID: content.ExampleID})
	})
}

func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && len(email) <= 254
}

func (s *Store) CreateAdmin(ctx context.Context, email, name, password string) error {
	if !validEmail(email) || strings.TrimSpace(name) == "" || len(name) > 100 {
		return domain.ErrInvalid
	}
	hash, err := auth.Password(password)
	if err != nil {
		return err
	}
	_, err = s.Q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, Name: name, PasswordHash: text(hash), Role: "admin"})
	return err
}

func (s *Store) Login(ctx context.Context, email, password, attemptKey, oldSession string) (string, error) {
	token, err := auth.Token()
	if err != nil {
		return "", err
	}
	accepted := false
	err = s.transaction(ctx, func(q *dbgen.Queries) error {
		if err := q.InitLoginAttempt(ctx, attemptKey); err != nil {
			return err
		}
		attempt, err := q.LockLoginAttempt(ctx, attemptKey)
		if err != nil {
			return err
		}
		if attempt.Locked {
			return nil
		}
		u, err := q.GetUserByEmail(ctx, email)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		hash := auth.DummyPasswordHash
		if err == nil && u.PasswordHash.Valid {
			hash = u.PasswordHash.String
		}
		valid := auth.VerifyPassword(hash, password)
		if !valid || err != nil || !u.IsActive {
			return q.FailLogin(ctx, attemptKey)
		}
		// Serialize session creation with account disable/role changes.
		u, err = q.LockUser(ctx, u.ID)
		if err != nil {
			return err
		}
		if !u.IsActive {
			return nil
		}
		if err = q.DeleteSession(ctx, oldSession); err != nil {
			return err
		}
		if err = q.CreateSession(ctx, dbgen.CreateSessionParams{ID: auth.TokenHash(token), UserID: u.ID}); err != nil {
			return err
		}
		accepted = true
		return q.ClearLogin(ctx, attemptKey)
	})
	if err != nil {
		return "", err
	}
	if !accepted {
		return "", domain.ErrLogin
	}
	return token, nil
}

func (s *Store) Invite(ctx context.Context, actorID int64, email, role string) (string, error) {
	if !validEmail(email) || (role != "admin" && role != "verifier") {
		return "", domain.ErrInvalid
	}
	token, err := auth.Token()
	if err != nil {
		return "", err
	}
	err = s.transaction(ctx, func(q *dbgen.Queries) error {
		actor, err := q.LockUser(ctx, actorID)
		if err != nil {
			return err
		}
		if !actor.IsActive || actor.Role != "admin" {
			return domain.ErrForbidden
		}
		return q.CreateInvitation(ctx, dbgen.CreateInvitationParams{Email: email, Role: role, TokenHash: auth.TokenHash(token), CreatedBy: actorID})
	})
	return token, err
}

func (s *Store) AcceptInvitation(ctx context.Context, token, name, password string) error {
	if len(token) != 43 || strings.TrimSpace(name) == "" || len(name) > 100 {
		return domain.ErrInvalid
	}
	hash, err := auth.Password(password)
	if err != nil {
		return domain.ErrInvalid
	}
	return s.transaction(ctx, func(q *dbgen.Queries) error {
		inv, err := q.ConsumeInvitation(ctx, auth.TokenHash(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalid
		}
		if err != nil {
			return err
		}
		actor, err := q.LockUser(ctx, inv.CreatedBy)
		if err != nil {
			return err
		}
		if !actor.IsActive || actor.Role != "admin" {
			return domain.ErrForbidden
		}
		_, err = q.CreateUser(ctx, dbgen.CreateUserParams{Email: inv.Email, Name: name, Role: inv.Role, PasswordHash: text(hash), CreatedBy: number(inv.CreatedBy)})
		return err
	})
}

func (s *Store) UpdateUser(ctx context.Context, actorID, targetID int64, role string, active bool) error {
	if actorID == targetID || (role != "admin" && role != "verifier") {
		return domain.ErrForbidden
	}
	return s.transaction(ctx, func(q *dbgen.Queries) error {
		// Lock in a stable order to avoid deadlocks between administrators.
		first, second := actorID, targetID
		if first > second {
			first, second = second, first
		}
		if _, err := q.LockUser(ctx, first); err != nil {
			return err
		}
		if _, err := q.LockUser(ctx, second); err != nil {
			return err
		}
		actor, err := q.GetUser(ctx, actorID)
		if err != nil {
			return err
		}
		if !actor.IsActive || actor.Role != "admin" {
			return domain.ErrForbidden
		}
		if err = q.UpdateUser(ctx, dbgen.UpdateUserParams{ID: targetID, Role: role, IsActive: active}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, targetID)
	})
}

func (s *Store) Purge(ctx context.Context) error {
	return s.transaction(ctx, func(q *dbgen.Queries) error {
		for _, f := range []func(context.Context) error{q.PurgeSubmissionHashes, q.PurgeRateLimits, q.PurgeBlocklist, q.PurgeSessions, q.PurgeLoginAttempts} {
			if err := f(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

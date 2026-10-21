//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"baso-lintang/internal/auth"
	"baso-lintang/internal/domain"
	"baso-lintang/internal/store/dbgen"
	"baso-lintang/internal/testutil"
	"github.com/jackc/pgx/v5"
)

func TestDatabaseInvariants(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	// All language fixtures are artificial test placeholders, not Lintang words.
	password := "password-uji-bukan-rahasia"
	if err := s.CreateAdmin(ctx, "admin@example.invalid", "Admin uji", password); err != nil {
		t.Fatal(err)
	}
	admin, err := s.Q.GetUserByEmail(ctx, "admin@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	in := domain.Submission{Headword: "kata-uji-1", Gloss: "arti pengujian", Explanation: "penjelasan uji", Name: "Nama uji", Contact: "kontak-uji@example.invalid", Consent: false, IPHash: "hash-uji", FingerprintHash: "hash-device"}
	if _, err = s.SaveSubmission(ctx, in); err != nil {
		t.Fatal(err)
	}
	if dup, err := s.SaveSubmission(ctx, in); err != nil || !dup {
		t.Fatalf("dedup: %v %v", dup, err)
	}
	rows, err := s.Q.SearchEntries(ctx, dbgen.SearchEntriesParams{Query: "katauji1", Direction: "lintang", PageLimit: 20})
	if err != nil || len(rows) != 0 {
		t.Fatalf("pending leaked: %v %v", rows, err)
	}
	queue, err := s.Q.Queue(ctx, dbgen.QueueParams{Status: "pending", PageLimit: 20})
	if err != nil || len(queue) != 1 {
		t.Fatalf("queue %v %v", queue, err)
	}
	r := domain.Review{SubmissionID: queue[0].ID, ReviewerID: admin.ID, Action: "approve", Content: in, Variants: []string{"kata-uji-varian"}}
	if err = s.Review(ctx, r); err != nil {
		t.Fatal("approve:", err)
	}
	if !errors.Is(s.Review(ctx, r), domain.ErrConflict) {
		t.Fatal("double approval accepted")
	}
	sub, err := s.Q.GetSubmission(ctx, r.SubmissionID)
	if err != nil {
		t.Fatal(err)
	}
	entryID := sub.ResultingEntryID.Int64
	for _, q := range []dbgen.SearchEntriesParams{
		{Query: "katauji1", Direction: "lintang", PageLimit: 20},
		{Query: "kataujvarian", Direction: "lintang", PageLimit: 20},
		{Query: "pengujian", Direction: "indonesia", PageLimit: 20},
		{Query: "pengujia", Direction: "indonesia", PageLimit: 20},
	} {
		hits, err := s.Q.SearchEntries(ctx, q)
		if err != nil || len(hits) != 1 {
			t.Fatalf("search %q: %v %v", q.Query, hits, err)
		}
	}
	credits, err := s.Q.PublicCredits(ctx, entryID)
	if err != nil || len(credits) != 0 {
		t.Fatal("unconsented name exposed", err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE submissions SET credit_consent=true WHERE id=$1", sub.ID); err != nil {
		t.Fatal(err)
	}
	credits, err = s.Q.PublicCredits(ctx, entryID)
	if err != nil || len(credits) != 1 {
		t.Fatal("consented credit missing", err)
	}
	for _, state := range []struct {
		visibility string
		sensitive  bool
	}{{"hidden", false}, {"published", true}} {
		if _, err = s.Pool.Exec(ctx, "UPDATE entries SET visibility=$2, is_sensitive=$3 WHERE id=$1", entryID, state.visibility, state.sensitive); err != nil {
			t.Fatal(err)
		}
		for _, direction := range []string{"lintang", "indonesia", "semua"} {
			rows, err := s.Q.SearchEntries(ctx, dbgen.SearchEntriesParams{Query: "pengujian", Direction: direction, PageLimit: 20})
			if err != nil || len(rows) != 0 {
				t.Fatal("hidden search leak", err)
			}
		}
		list, err := s.Q.ListEntries(ctx, dbgen.ListEntriesParams{PageLimit: 20})
		if err != nil || len(list) != 0 {
			t.Fatal("hidden list leak", err)
		}
		if _, err = s.Q.PublicEntry(ctx, "katauji1-"+fmt.Sprint(sub.ID)); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("hidden detail leak", err)
		}
		if v, err := s.Q.PublicSenses(ctx, entryID); err != nil || len(v) != 0 {
			t.Fatal("hidden sense leak", err)
		}
		if v, err := s.Q.PublicSpellings(ctx, entryID); err != nil || len(v) != 0 {
			t.Fatal("hidden spelling leak", err)
		}
		if v, err := s.Q.PublicExamples(ctx, entryID); err != nil || len(v) != 0 {
			t.Fatal("hidden example leak", err)
		}
		if v, err := s.Q.PublicCredits(ctx, entryID); err != nil || len(v) != 0 {
			t.Fatal("hidden credit leak", err)
		}
	}
	t.Run("BR4", func(t *testing.T) {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "INSERT INTO entries(slug,headword,normalized) VALUES('kata-uji-tidak-lengkap','kata-uji-2','katauji2')"); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err == nil {
			t.Fatal("entry without spelling and sense committed")
		}
		if _, err = s.Pool.Exec(ctx, "DELETE FROM spellings WHERE entry_id=$1", entryID); err == nil {
			t.Fatal("last primary spelling removed")
		}
		if _, err = s.Pool.Exec(ctx, "DELETE FROM senses WHERE entry_id=$1", entryID); err == nil {
			t.Fatal("last sense removed")
		}
	})
	t.Run("self approval", func(t *testing.T) {
		own := in
		own.Headword = "kata-uji-sendiri"
		own.UserID = admin.ID
		own.SessionHash = "old-session"
		if _, err = s.SaveSubmission(ctx, own); err != nil {
			t.Fatal(err)
		}
		q, err := s.Q.Queue(ctx, dbgen.QueueParams{Status: "pending", PageLimit: 20})
		if err != nil {
			t.Fatal(err)
		}
		err = s.Review(ctx, domain.Review{SubmissionID: q[0].ID, ReviewerID: admin.ID, SessionHash: "new-session", Action: "approve", Content: own})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatal("self approval allowed", err)
		}
	})
	t.Run("review transitions and rollback", func(t *testing.T) {
		candidate := in
		candidate.Headword = "kata-uji-perbaikan"
		if _, err := s.SaveSubmission(ctx, candidate); err != nil {
			t.Fatal(err)
		}
		pending, err := s.Q.Queue(ctx, dbgen.QueueParams{Status: "pending", Query: candidate.Headword, PageLimit: 20})
		if err != nil || len(pending) != 1 {
			t.Fatal("pending fixture", err)
		}
		review := domain.Review{SubmissionID: pending[0].ID, ReviewerID: admin.ID, Action: "needs_fix", Note: "Perjelas arti uji"}
		if err := s.Review(ctx, review); err != nil {
			t.Fatal(err)
		}
		review.Action = "approve"
		review.Content = candidate
		if !errors.Is(s.Review(ctx, review), domain.ErrConflict) {
			t.Fatal("needs_fix approved without requeue")
		}
		review.Action = "resubmit"
		review.Content.Gloss = "arti uji diperbaiki"
		if err := s.Review(ctx, review); err != nil {
			t.Fatal(err)
		}
		review.Action = "merge"
		review.TargetEntryID = 9999999
		if err := s.Review(ctx, review); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("missing merge target accepted", err)
		}
		unchanged, err := s.Q.GetSubmission(ctx, review.SubmissionID)
		if err != nil || unchanged.Status != "pending" {
			t.Fatal("failed merge changed status", err)
		}
		review.TargetEntryID = entryID
		if err := s.Review(ctx, review); err != nil {
			t.Fatal("merge", err)
		}
		merged, err := s.Q.GetSubmission(ctx, review.SubmissionID)
		if err != nil || merged.Status != "approved" || merged.ResultingEntryID.Int64 != entryID {
			t.Fatal("merge status", err)
		}
		candidate.Headword = "kata-uji-tolak"
		if _, err := s.SaveSubmission(ctx, candidate); err != nil {
			t.Fatal(err)
		}
		pending, err = s.Q.Queue(ctx, dbgen.QueueParams{Status: "pending", Query: candidate.Headword, PageLimit: 20})
		if err != nil || len(pending) != 1 {
			t.Fatal("reject fixture", err)
		}
		review = domain.Review{SubmissionID: pending[0].ID, ReviewerID: admin.ID, Action: "reject"}
		if !errors.Is(s.Review(ctx, review), domain.ErrInvalid) {
			t.Fatal("rejection without reason accepted")
		}
		review.Note = "Data uji tidak digunakan"
		if err := s.Review(ctx, review); err != nil {
			t.Fatal(err)
		}
		rejected, err := s.Q.GetSubmission(ctx, review.SubmissionID)
		if err != nil || rejected.Status != "rejected" || rejected.ResultingEntryID.Valid {
			t.Fatal("rejected became entry", err)
		}
	})
	t.Run("concurrent dedup", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Go(func() {
				dup := in
				dup.Headword = "kata-uji-duplikat"
				_, err := s.SaveSubmission(ctx, dup)
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		var count int
		if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM submissions WHERE headword='kata-uji-duplikat'").Scan(&count); err != nil || count != 1 {
			t.Fatal("concurrent duplicates", count, err)
		}
	})
	t.Run("rate limit atomic", func(t *testing.T) {
		var wg sync.WaitGroup
		allowed := make(chan bool, 20)
		errs := make(chan error, 20)
		for range 20 {
			wg.Go(func() { ok, err := s.Allow(ctx, "rate-test", 10, 30); allowed <- ok; errs <- err })
		}
		wg.Wait()
		close(allowed)
		close(errs)
		count := 0
		for ok := range allowed {
			if ok {
				count++
			}
		}
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if count != 10 {
			t.Fatal("rate race", count)
		}
	})
	t.Run("BR7 purge", func(t *testing.T) {
		if _, err = s.Pool.Exec(ctx, "UPDATE submissions SET created_at=now()-interval '91 days' WHERE id=$1", sub.ID); err != nil {
			t.Fatal(err)
		}
		if err = s.Purge(ctx); err != nil {
			t.Fatal(err)
		}
		old, err := s.Q.GetSubmission(ctx, sub.ID)
		if err != nil || old.IpHash.Valid || old.FingerprintHash.Valid {
			t.Fatal("hash not purged", err)
		}
	})
	t.Run("invitation login lockout revocation", func(t *testing.T) {
		token, err := s.Invite(ctx, admin.ID, "verifier@example.invalid", "verifier")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.AcceptInvitation(ctx, token, "Verifikator uji", password); err != nil {
			t.Fatal(err)
		}
		if err = s.AcceptInvitation(ctx, token, "Verifikator uji", password); err == nil {
			t.Fatal("invitation replay")
		}
		for range 5 {
			if _, err = s.Login(ctx, "verifier@example.invalid", "wrong", "attempt-test", ""); !errors.Is(err, domain.ErrLogin) {
				t.Fatal(err)
			}
		}
		if _, err = s.Login(ctx, "verifier@example.invalid", password, "attempt-test", ""); !errors.Is(err, domain.ErrLogin) {
			t.Fatal("lockout bypass", err)
		}
		if _, err = s.Pool.Exec(ctx, "UPDATE login_attempts SET locked_until=now()-interval '1 second' WHERE key_hash=$1", "attempt-test"); err != nil {
			t.Fatal(err)
		}
		session, err := s.Login(ctx, "verifier@example.invalid", password, "attempt-test", "")
		if err != nil {
			t.Fatal(err)
		}
		u, err := s.Q.SessionUser(ctx, auth.TokenHash(session))
		if err != nil {
			t.Fatal(err)
		}
		newSession, err := s.Login(ctx, u.Email, password, "attempt-test", auth.TokenHash(session))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Q.SessionUser(ctx, auth.TokenHash(session)); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("old session survived rotation")
		}
		if _, err = s.Invite(ctx, u.ID, "forbidden@example.invalid", "admin"); !errors.Is(err, domain.ErrForbidden) {
			t.Fatal("verifier invited admin")
		}
		if err = s.UpdateUser(ctx, admin.ID, u.ID, "verifier", false); err != nil {
			t.Fatal(err)
		}
		if _, err = s.Q.SessionUser(ctx, auth.TokenHash(newSession)); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("disabled session survived")
		}
		expired, err := s.Invite(ctx, admin.ID, "expired@example.invalid", "verifier")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Pool.Exec(ctx, "UPDATE invitations SET expires_at=$2 WHERE token_hash=$1", auth.TokenHash(expired), time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err = s.AcceptInvitation(ctx, expired, "Uji", password); err == nil {
			t.Fatal("expired invitation accepted")
		}
	})
}

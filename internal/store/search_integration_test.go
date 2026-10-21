//go:build integration

package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"baso-lintang/internal/store/dbgen"
	"baso-lintang/internal/testutil"
	"github.com/jackc/pgx/v5"
)

type captureQuery struct {
	dbgen.DBTX
	sql  string
	args []any
}

func (c *captureQuery) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.sql = sql
	c.args = args
	return c.DBTX.Query(ctx, sql, args...)
}

func TestSearch50000Entries(t *testing.T) {
	if testing.Short() {
		t.Skip("large synthetic search fixture")
	}
	s := testutil.Database(t)
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Synthetic placeholders only; these are not Lintang vocabulary.
	for _, statement := range []string{
		`INSERT INTO entries(slug,headword,normalized)
		 SELECT 'kata-uji-'||n, 'kata-uji-'||n, 'katauji'||n FROM generate_series(1,50000) n`,
		`INSERT INTO spellings(entry_id,form,normalized,is_primary)
		 SELECT id, headword, normalized, true FROM entries`,
		`INSERT INTO senses(entry_id,gloss_id,explanation)
		 SELECT id, 'arti uji '||id, 'penjelasan buatan untuk pengujian' FROM entries`,
	} {
		if _, err = tx.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	capture := &captureQuery{DBTX: s.Pool}
	q := dbgen.New(capture)
	for _, tc := range []struct{ query, direction string }{{"katauji12345", "lintang"}, {"katau12345", "lintang"}, {"arti uji 12345", "indonesia"}, {"penjelasan", "indonesia"}} {
		t.Run(tc.direction+"/"+tc.query, func(t *testing.T) {
			params := dbgen.SearchEntriesParams{Query: tc.query, Direction: tc.direction, PageLimit: 20}
			if _, err := q.SearchEntries(ctx, params); err != nil {
				t.Fatal(err)
			}
			// The SQL is captured verbatim from generated sqlc code, never user input.
			rows, err := s.Pool.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+capture.sql, capture.args...)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var line string
				if err = rows.Scan(&line); err != nil {
					t.Fatal(err)
				}
				t.Log(line)
			}
			if err = rows.Err(); err != nil {
				t.Fatal(err)
			}
			rows.Close()
			durations := make([]time.Duration, 30)
			for i := range durations {
				start := time.Now()
				hits, err := q.SearchEntries(ctx, params)
				durations[i] = time.Since(start)
				if err != nil || len(hits) == 0 {
					t.Fatal("search", err)
				}
				if (tc.query == "katauji12345" || tc.query == "arti uji 12345") && hits[0].Headword != "kata-uji-12345" {
					t.Fatal("exact match must precede fuzzy matches")
				}
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			p95 := durations[28]
			t.Logf("50,000 synthetic entries, 30 serial samples, p95=%s", p95)
			if p95 >= 300*time.Millisecond {
				t.Errorf("local p95 exceeds 300ms: %s", p95)
			}
		})
	}
}

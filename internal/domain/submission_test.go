package domain

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeSubmissionRepo struct {
	calls                  *[]string
	blocked, denied, saved bool
	duplicate              bool
}

func (f *fakeSubmissionRepo) Blocked(context.Context, string) (bool, error) {
	*f.calls = append(*f.calls, "block")
	return f.blocked, nil
}
func (f *fakeSubmissionRepo) Allow(_ context.Context, key string, h, d int) (bool, error) {
	*f.calls = append(*f.calls, "rate:"+strings.Split(key, ":")[0])
	return !f.denied, nil
}
func (f *fakeSubmissionRepo) SaveSubmission(context.Context, Submission) (bool, error) {
	*f.calls = append(*f.calls, "save")
	f.saved = true
	return f.duplicate, nil
}

type fakeCaptcha struct {
	calls *[]string
	valid bool
	err   error
}

func (f fakeCaptcha) Verify(context.Context, string) (bool, error) {
	*f.calls = append(*f.calls, "captcha")
	return f.valid, f.err
}

func TestSubmissionOrderAndQuarantine(t *testing.T) {
	// These are explicitly artificial fixtures, not Lintang vocabulary.
	for _, tc := range []struct {
		name                   string
		blocked, denied, valid bool
		outage                 bool
		honey, word            string
		want                   []string
		save                   bool
	}{
		{"accepted", false, false, true, false, "", "kata-uji-1", []string{"block", "rate:ip", "rate:device", "captcha", "save"}, true},
		{"blocked", true, false, true, false, "", "kata-uji-1", []string{"block"}, false},
		{"rate", false, true, true, false, "", "kata-uji-1", []string{"block", "rate:ip"}, false},
		{"captcha", false, false, false, false, "", "kata-uji-1", []string{"block", "rate:ip", "rate:device", "captcha"}, false},
		{"honeypot", false, false, true, false, "spam", "kata-uji-1", []string{"block", "rate:ip", "rate:device", "captcha"}, false},
		{"validation", false, false, true, false, "", "", []string{"block", "rate:ip", "rate:device", "captcha"}, false},
		{"outage", false, false, false, true, "", "kata-uji-1", []string{"block", "rate:ip", "rate:device", "captcha", "rate:outage-ip", "rate:outage-device", "save"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			repo := &fakeSubmissionRepo{calls: &calls, blocked: tc.blocked, denied: tc.denied}
			captcha := fakeCaptcha{calls: &calls, valid: tc.valid}
			if tc.outage {
				captcha.err = errors.New("unavailable")
			}
			svc := SubmissionService{Repo: repo, Captcha: captcha, Hour: 10, Day: 30, OutageHour: 2, OutageDay: 5}
			_, err := svc.Submit(context.Background(), Submission{Headword: tc.word, Gloss: "arti uji"}, "token", tc.honey)
			if tc.save && err != nil {
				t.Fatal(err)
			}
			if !tc.save && err == nil {
				t.Fatal("expected rejection")
			}
			if !reflect.DeepEqual(calls, tc.want) || repo.saved != tc.save {
				t.Fatalf("calls=%v saved=%v", calls, repo.saved)
			}
		})
	}
}

func TestValidationAndNormalization(t *testing.T) {
	n := Normalizer{}
	if got := n.Normalize(" KÁTA-UJI! "); got != "katauji" {
		t.Fatalf("normalization: %q", got)
	}
	n = Normalizer{Replacements: []string{"x", "z"}}
	if n.Normalize("x") != "z" {
		t.Fatal("configured replacement ignored")
	}
	for _, word := range []string{"", strings.Repeat("a", 61), "https://example.invalid", "a@example.invalid", "example.invalid", "uji\x00"} {
		if Validate(Submission{Headword: word, Gloss: "arti uji"}) == nil {
			t.Fatalf("accepted %q", word)
		}
	}
	if err := Validate(Submission{Headword: strings.Repeat("é", 60), Gloss: "arti uji"}); err != nil {
		t.Fatal("must count runes", err)
	}
}

func TestCannotApproveOwnSubmission(t *testing.T) {
	for _, action := range []string{"approve", "merge"} {
		r := Review{Action: action, ReviewerID: 7, SessionHash: "session-new"}
		if !errors.Is(CanReview(7, "session-old", r), ErrForbidden) {
			t.Fatal("own user allowed after session rotation")
		}
		if !errors.Is(CanReview(0, "session-new", r), ErrForbidden) {
			t.Fatal("own session allowed")
		}
		if err := CanReview(8, "other", r); err != nil {
			t.Fatal(err)
		}
	}
}

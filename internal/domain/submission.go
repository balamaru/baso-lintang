package domain

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var (
	ErrInvalid   = errors.New("isian tidak valid")
	ErrLimited   = errors.New("batas kiriman tercapai, silakan coba lagi nanti")
	ErrCaptcha   = errors.New("verifikasi CAPTCHA gagal, silakan coba lagi")
	ErrForbidden = errors.New("tindakan tidak diizinkan")
	ErrConflict  = errors.New("kiriman sudah ditinjau")
	ErrLogin     = errors.New("email atau password tidak sesuai, atau akses sementara dibatasi")
	ErrNotFound  = errors.New("data tidak ditemukan")
)

type Submission struct {
	Headword, Gloss, Explanation, POS, ExampleLintang, ExampleID string
	Name, Contact                                                string
	Consent                                                      bool
	IPHash, FingerprintHash, SessionHash                         string
	UserID                                                       int64
}

// Normalizer changes search keys only; the original spelling is preserved.
type Normalizer struct{ Replacements []string }

func (n Normalizer) Normalize(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(strings.TrimSpace(s))) {
		if unicode.Is(unicode.Mn, r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	s = strings.Join(strings.Fields(b.String()), " ")
	if len(n.Replacements) > 0 {
		s = strings.NewReplacer(n.Replacements...).Replace(s)
	}
	return norm.NFC.String(s)
}

func Validate(s Submission) error {
	if strings.TrimSpace(s.Headword) == "" || strings.TrimSpace(s.Gloss) == "" {
		return ErrInvalid
	}
	fields := []struct {
		value string
		max   int
	}{
		{s.Headword, 60}, {s.Gloss, 1000}, {s.Explanation, 1000}, {s.POS, 60},
		{s.ExampleLintang, 1000}, {s.ExampleID, 1000}, {s.Name, 100}, {s.Contact, 254},
	}
	for _, f := range fields {
		if !utf8.ValidString(f.value) || utf8.RuneCountInString(f.value) > f.max {
			return ErrInvalid
		}
		for _, r := range f.value {
			if unicode.IsControl(r) {
				return ErrInvalid
			}
		}
	}
	word := strings.ToLower(s.Headword)
	if strings.ContainsAny(word, "@:/\\") || strings.Contains(word, "www.") || strings.Contains(word, ".") {
		return ErrInvalid
	}
	return nil
}

type SubmissionRepository interface {
	Blocked(context.Context, string) (bool, error)
	Allow(context.Context, string, int, int) (bool, error)
	SaveSubmission(context.Context, Submission) (bool, error)
}

type Captcha interface {
	// Verify distinguishes invalid tokens from service unavailability.
	Verify(context.Context, string) (bool, error)
}

type SubmissionService struct {
	Repo                             SubmissionRepository
	Captcha                          Captcha
	Hour, Day, OutageHour, OutageDay int
}

func (s SubmissionService) Submit(ctx context.Context, in Submission, token, honeypot string) (bool, error) {
	blocked, err := s.Repo.Blocked(ctx, in.IPHash)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, ErrLimited
	}
	for _, key := range []string{"ip:" + in.IPHash, "device:" + in.FingerprintHash} {
		ok, err := s.Repo.Allow(ctx, key, s.Hour, s.Day)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, ErrLimited
		}
	}
	ok, err := s.Captcha.Verify(ctx, token)
	if err != nil {
		// The documented outage path remains quarantined and uses tighter limits.
		for _, key := range []string{"outage-ip:" + in.IPHash, "outage-device:" + in.FingerprintHash} {
			allowed, limitErr := s.Repo.Allow(ctx, key, s.OutageHour, s.OutageDay)
			if limitErr != nil {
				return false, limitErr
			}
			if !allowed {
				return false, ErrLimited
			}
		}
	} else if !ok {
		return false, ErrCaptcha
	}
	if honeypot != "" {
		return false, ErrInvalid
	}
	if err := Validate(in); err != nil {
		return false, err
	}
	return s.Repo.SaveSubmission(ctx, in)
}

type Review struct {
	SubmissionID, ReviewerID, TargetEntryID int64
	SessionHash, Action, Note               string
	Content                                 Submission
	Variants                                []string
}

func ValidateReview(r Review) error {
	if r.Action != "approve" && r.Action != "merge" && r.Action != "reject" && r.Action != "needs_fix" && r.Action != "resubmit" {
		return ErrInvalid
	}
	if len(r.Note) > 4000 || ((r.Action == "reject" || r.Action == "needs_fix") && strings.TrimSpace(r.Note) == "") {
		return ErrInvalid
	}
	if r.Action == "approve" || r.Action == "merge" || r.Action == "resubmit" {
		if err := Validate(r.Content); err != nil {
			return err
		}
		if len(r.Variants) > 20 {
			return ErrInvalid
		}
		for _, word := range r.Variants {
			v := r.Content
			v.Headword = word
			if err := Validate(v); err != nil {
				return err
			}
		}
		if r.Action == "merge" && r.TargetEntryID <= 0 {
			return ErrInvalid
		}
	}
	return nil
}

func CanReview(authorID int64, authorSession string, r Review) error {
	if (r.Action == "approve" || r.Action == "merge") &&
		((authorID != 0 && authorID == r.ReviewerID) || (authorSession != "" && authorSession == r.SessionHash)) {
		return ErrForbidden
	}
	return nil
}

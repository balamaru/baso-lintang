package auth

import (
	"strings"
	"testing"
)

func TestPasswordAndTokens(t *testing.T) {
	password := "password-uji-bukan-rahasia"
	h, err := Password(password)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, password) || VerifyPassword(h, "salah") {
		t.Fatal("password verification failed")
	}
	if VerifyPassword(strings.Replace(h, "m=65536", "m=999999999", 1), password) {
		t.Fatal("unsafe hash parameters accepted")
	}
	if _, err := Password("pendek"); err == nil {
		t.Fatal("short password allowed")
	}
	a, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) != 43 || TokenHash(a) == a {
		t.Fatal("invalid token generation")
	}
	if Hash("salt", "ip", "value") == Hash("salt", "device", "value") {
		t.Fatal("hash purposes must be distinct")
	}
}

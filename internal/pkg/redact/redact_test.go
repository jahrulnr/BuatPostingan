package redact_test

import (
	"context"
	"strings"
	"testing"

	"buatpostingan/internal/pkg/redact"
)

func TestRedactOpenAIAndJWT(t *testing.T) {
	in := "key sk-abcdefghijklmnopqrstuvwxyz and jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature_here_ok"
	out := redact.RedactText(in)
	if strings.Contains(out, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("openai key not redacted: %s", out)
	}
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiJ9") {
		t.Fatalf("jwt not redacted: %s", out)
	}
	if !strings.Contains(out, "[REDACTED_OPENAI_KEY]") {
		t.Fatalf("missing OPENAI marker: %s", out)
	}
	if !strings.Contains(out, "[REDACTED_JWT]") {
		t.Fatalf("missing JWT marker: %s", out)
	}
}

func TestRedactBearerAndGeneric(t *testing.T) {
	in := "Authorization: Bearer abcdefghijklmnop password=supersecretvalue"
	out := redact.RedactText(in)
	if strings.Contains(out, "abcdefghijklmnop") || strings.Contains(out, "supersecretvalue") {
		t.Fatalf("not redacted: %s", out)
	}
}

func TestRedactMD5NeedsContext(t *testing.T) {
	plain := "checksum deadbeefdeadbeefdeadbeefdeadbeef"
	if got := redact.RedactText(plain); strings.Contains(got, "[REDACTED_MD5_HEX]") {
		t.Fatalf("md5 without keyword should pass: %s", got)
	}
	withCtx := "md5 hash deadbeefdeadbeefdeadbeefdeadbeef"
	if got := redact.RedactText(withCtx); !strings.Contains(got, "[REDACTED_MD5_HEX]") {
		t.Fatalf("md5 with keyword should redact: %s", got)
	}
}

func TestSecretsAdapter(t *testing.T) {
	s := redact.New()
	out, err := s.Redact(context.Background(), "sk-abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[REDACTED_OPENAI_KEY]") {
		t.Fatalf("got %s", out)
	}
}

func TestRedactMD5RuneBoundaryLookback(t *testing.T) {
	// Place a multi-byte rune so lookback lands mid-sequence and steps to RuneStart.
	prefix := strings.Repeat("x", 35) + "й" // 2-byte rune near the lookback edge
	in := prefix + "md5 deadbeefdeadbeefdeadbeefdeadbeef"
	got := redact.RedactText(in)
	if !strings.Contains(got, "[REDACTED_MD5_HEX]") {
		t.Fatalf("got %s", got)
	}
}

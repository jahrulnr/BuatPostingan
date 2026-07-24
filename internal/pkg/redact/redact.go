// Package redact scrubs API keys / JWTs / private keys from user text (AIPedia WebchatRedactSecrets).
package redact

import (
	"context"
	"regexp"
	"unicode/utf8"
)

// Secrets implements domain service.SecretRedactor.
type Secrets struct{}

// New returns a SecretRedactor adapter around RedactText.
func New() Secrets { return Secrets{} }

func (Secrets) Redact(_ context.Context, text string) (string, error) {
	return RedactText(text), nil
}

type rule struct {
	kind    string
	re      *regexp.Regexp
	context bool
}

var rules = []rule{
	{kind: "PRIVATE_KEY", re: regexp.MustCompile(`-----BEGIN[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----[\s\S]*?-----END[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----`)},
	{kind: "JWT", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)},
	{kind: "OPENAI_KEY", re: regexp.MustCompile(`\bsk-(?:proj|svcacct|admin)-[A-Za-z0-9_-]{20,}\b`)},
	{kind: "OPENAI_KEY", re: regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`)},
	{kind: "GROQ_KEY", re: regexp.MustCompile(`\bgsk_[A-Za-z0-9]{20,}\b`)},
	{kind: "GITHUB_PAT", re: regexp.MustCompile(`\b(?:ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{20,})\b`)},
	{kind: "AWS_ACCESS_KEY", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{kind: "BEARER", re: regexp.MustCompile(`(?i)(?:authorization\s*[:=]\s*)?bearer\s+[A-Za-z0-9\-._~+/=]+`)},
	{kind: "BASIC_AUTH", re: regexp.MustCompile(`(?i)(?:authorization\s*[:=]\s*)?basic\s+[A-Za-z0-9+/]{8,}={0,3}`)},
	{kind: "BCRYPT", re: regexp.MustCompile(`\$2[aby]?\$\d{2}\$[.\/A-Za-z0-9]{53}`)},
	{kind: "GENERIC_ASSIGNED", re: regexp.MustCompile(`(?i)(?:api[_-]?key|secret|password|passwd|token|credential)\s*[:=]\s*['"]?[^\s'"&,;]{8,}`)},
	{kind: "MD5_HEX", re: regexp.MustCompile(`\b[a-fA-F0-9]{32}\b`), context: true},
	{kind: "SHA256_HEX", re: regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`), context: true},
}

var contextKeyword = regexp.MustCompile(`(?i)(pass|md5|hash|secret|token)`)

// RedactText applies AIPedia secret patterns and returns scrubbed text.
func RedactText(text string) string {
	out := text
	for _, r := range rules {
		matches := r.re.FindAllStringIndex(out, -1)
		if len(matches) == 0 {
			continue
		}
		// Replace from end so earlier offsets stay valid.
		for i := len(matches) - 1; i >= 0; i-- {
			start, end := matches[i][0], matches[i][1]
			if r.context && !contextHasSecretKeyword(out, start, 40) {
				continue
			}
			token := "[REDACTED_" + r.kind + "]"
			out = out[:start] + token + out[end:]
		}
	}
	return out
}

func contextHasSecretKeyword(text string, start, lookback int) bool {
	from := start - lookback
	if from < 0 {
		from = 0
	}
	// Ensure we slice on rune boundaries.
	for from > 0 && !utf8.RuneStart(text[from]) {
		from--
	}
	slice := text[from:start]
	return contextKeyword.MatchString(slice)
}

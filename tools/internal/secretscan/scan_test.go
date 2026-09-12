package secretscan

import (
	"strings"
	"testing"
)

func TestRedactFindsKnownPatterns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"aws", "key = AKIAABCDEFGHIJKLMNOP", "aws-access-key"},
		{"openai-style", "token: sk-abcdefghijklmnopqrstuvwxyz123456", "generic-api-key"},
		{"github", "auth ghp_abcdefghijklmnopqrstuvwxyz1234", "github-token"},
		{"slack", "xoxb-1234567890-abcdefghij", "slack-token"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFmbmMLuA5A", "jwt"},
		{"private-key", "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAK...\n-----END RSA PRIVATE KEY-----", "private-key-block"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			redacted, found := Redact(c.text)
			if len(found) == 0 {
				t.Fatalf("esperava detectar %q em %q", c.want, c.text)
			}
			if found[0] != c.want {
				t.Fatalf("got %v, want %s", found, c.want)
			}
			if strings.Contains(redacted, c.text) {
				t.Fatalf("texto original não foi redigido: %s", redacted)
			}
		})
	}
}

func TestRedactLeavesCleanTextUntouched(t *testing.T) {
	text := "isso é um texto de documentação normal, sem segredos."
	redacted, found := Redact(text)
	if len(found) != 0 {
		t.Fatalf("não esperava achados, got %v", found)
	}
	if redacted != text {
		t.Fatalf("texto limpo foi alterado: %q", redacted)
	}
}

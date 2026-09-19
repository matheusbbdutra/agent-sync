package trackdocs

import (
	"strings"
	"testing"
)

// TestExtractPostTextsRejectsMalformed verifies that extractPostTexts handles
// malformed JSON safely. The forum endpoint returns well-formed JSON in
// practice, but a defensive check keeps us from panicking on schema drift.
func TestExtractPostTextsRejectsMalformed(t *testing.T) {
	_, err := extractPostTexts("not json at all")
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

// TestExtractPostTextsConcatenates verifies the simple happy path: cooked
// fields from each post are concatenated into one string. We use the
// structure observed on 2026-09-19 in the live Cursor forum topic #147216.
func TestExtractPostTextsConcatenates(t *testing.T) {
	body := `{
		"post_stream": {
			"posts": [
				{"cooked": "<p>hello world</p>"},
				{"cooked": "<p>second post about tokens</p>"}
			]
		}
	}`
	got, err := extractPostTexts(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "hello world") || !strings.Contains(got, "second post about tokens") {
		t.Fatalf("expected both posts in output, got: %q", got)
	}
}

// TestCursorForumKnownFalsePositive is the regression test we wrote after
// deanrie's post #6 (2026-06-28). It contains "we've shipped other
// improvements to hook payloads" referring to model fields, NOT tokens.
// Our Positive regex must NOT match this. Our Negative regex must match it
// so the human-runner sees the tolerated phrase in the [OK] line.
func TestCursorForumKnownFalsePositive(t *testing.T) {
	src, ok := SourceByName("cursor-forum-147216")
	if !ok {
		t.Fatal("source cursor-forum-147216 not found")
	}
	// Real excerpt from deanrie's post #6 (2026-06-28), captured on 2026-09-19.
	body := "Hey, thanks for the ping. Honest update: we haven't added token usage metadata to hook inputs yet, so current hook payloads don't include it. I can't share an ETA. I logged this as a feature request and I see the demand. If anything changes around token fields in hooks, I'll post an update here. Related: we've shipped other improvements to hook payloads in the meantime, like more accurate model fields, but token breakdown still isn't included."
	match, hits := matchRegexes(src, body)
	if match {
		t.Fatalf("POSITIVE regex false-matched deanrie's post #6. Hits: %v\nBody: %q", hits, body)
	}
	if !hasKnownFalsePositive(src, body) {
		t.Fatalf("expected Negative regex to flag 'shipped other improvements' as known false-positive; got no hits")
	}
}

// TestCursorForumTruePositiveSimulated verifies that the regex correctly
// matches a hypothetical "we have shipped token usage metadata" announcement.
func TestCursorForumTruePositiveSimulated(t *testing.T) {
	src, ok := SourceByName("cursor-forum-147216")
	if !ok {
		t.Fatal("source cursor-forum-147216 not found")
	}
	body := "Good news everyone — we've shipped token usage metadata to hook inputs as of today. input and output counts are now in the postToolUse payload."
	match, _ := matchRegexes(src, body)
	if !match {
		t.Fatalf("expected POSITIVE match on simulated shipped-token announcement; body: %q", body)
	}
}

// TestAntigravityDocsNoMatch verifies that the current Antigravity docs do
// not match positive regex (sanity check against the live URL we fetched on
// 2026-09-19). We use a small excerpt of the docs to keep this offline-safe.
func TestAntigravityDocsNoMatch(t *testing.T) {
	src, ok := SourceByName("antigravity-hooks-docs")
	if !ok {
		t.Fatal("source antigravity-hooks-docs not found")
	}
	body := "All hooks receive the following system metadata fields in their input payload on stdin: conversationId, workspacePaths, transcriptPath, artifactDirectoryPath, modelName."
	match, _ := matchRegexes(src, body)
	if match {
		t.Fatalf("expected Antigravity docs Common Fields to NOT match positive regex; got match")
	}
}

// TestCodeOfMapping verifies that CodeOf returns the embedded code for our
// own exit errors and 1 for any other error.
func TestCodeOfMapping(t *testing.T) {
	if got := CodeOf(nil); got != 0 {
		t.Errorf("CodeOf(nil) = %d, want 0", got)
	}
	if got := CodeOf(newExitError(matchCode, "x")); got != matchCode {
		t.Errorf("CodeOf(match) = %d, want %d", got, matchCode)
	}
	if got := CodeOf(newExitError(netTimeoutCode, "x")); got != netTimeoutCode {
		t.Errorf("CodeOf(net) = %d, want %d", got, netTimeoutCode)
	}
	if got := CodeOf(newExitError(noMatchCode, "x")); got != noMatchCode {
		t.Errorf("CodeOf(noMatch) = %d, want %d", got, noMatchCode)
	}
	if got := CodeOf(&fakeErr{"boom"}); got != 1 {
		t.Errorf("CodeOf(other error) = %d, want 1 (default error code)", got)
	}
}

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

// TestParseFlagsSnapshotDir verifies that --snapshot-dir is parsed and
// preserved by parseFlags.
func TestParseFlagsSnapshotDir(t *testing.T) {
	o := parseFlags([]string{"--snapshot-dir", "/tmp/foo"})
	if o.snapshotDir != "/tmp/foo" {
		t.Fatalf("snapshotDir = %q, want %q", o.snapshotDir, "/tmp/foo")
	}
}

// TestParseFlagsDefaults verifies parseFlags with no args returns an empty
// fsOpts (snapshotDir stays "" so the caller falls back to default).
func TestParseFlagsDefaults(t *testing.T) {
	o := parseFlags(nil)
	if o.snapshotDir != "" {
		t.Fatalf("expected empty snapshotDir with no flags; got %q", o.snapshotDir)
	}
}

// TestAllSourcesConfigured is a smoke-level guardrail that confirms we still
// have all 4 expected sources wired up. If a future refactor drops one,
// this test will fail and prompt the human to update the playbook.
func TestAllSourcesConfigured(t *testing.T) {
	want := []string{
		"cursor-hooks-docs",
		"cursor-forum-147216",
		"antigravity-hooks-docs",
		"antigravity-sdk-issues",
	}
	for _, name := range want {
		if _, ok := SourceByName(name); !ok {
			t.Errorf("expected source %q to be configured; missing", name)
		}
	}
}

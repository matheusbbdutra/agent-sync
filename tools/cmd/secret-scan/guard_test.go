package main

import (
	"os"
	"strings"
	"testing"
)

func TestGuardParityWithLegacyShell(t *testing.T) {
	cases := []struct {
		name       string
		payload    string
		wantDeny   bool
		wantRedact string // "", "total", "inline"
	}{
		{"pre_no_secret", "{\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"ls -la\"}}", false, ""},
		{"post_no_secret", "{\"tool_output\":\"total 4\\ndrwxr-xr-x 2 user user 4096 sep 25 12:00 .\\n\"}", false, ""},
		{"pre_read_dotenv", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/home/u/proj/.env\"}}", true, ""},
		{"pre_read_aws_credentials", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/home/u/.aws/credentials\"}}", true, ""},
		{"pre_read_ssh_id", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/home/u/.ssh/id_rsa\"}}", true, ""},
		{"pre_read_aws_dir", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/home/u/.aws\"}}", true, ""},
		{"pre_bash_cat_zshrc", "{\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"cat ~/.zshrc\"}}", true, ""},
		{"pre_edit_pem", "{\"tool_name\":\"Edit\",\"tool_input\":{\"file_path\":\"/tmp/server.pem\"}}", true, ""},
		{"pre_grep_aws_dir", "{\"tool_name\":\"Grep\",\"tool_input\":{\"path\":\"/home/u/.aws\",\"pattern\":\"x\"}}", true, ""},
		{"pre_glob_secrets", "{\"tool_name\":\"Glob\",\"tool_input\":{\"path\":\"/proj/secrets\",\"pattern\":\"*\"}}", true, ""},
		{"pre_read_legit", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/tmp/file.txt\"}}", false, ""},
		{"pre_read_aws_config", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/home/u/.aws/config\"}}", true, ""},
		{"pre_newline_check", "{\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"ls\"}}", false, ""},
		{"post_read_denied_redact_total", "{\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"/proj/.env\"},\"tool_output\":\"DATABASE_URL=postgres://u:p@h/db\"}", false, "total"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deny, _ := Check([]byte(c.payload))
			res := Redact([]byte(c.payload))
			if c.wantDeny && !deny {
				t.Fatalf("esperava deny, payload=%s", c.payload)
			}
			if !c.wantDeny && c.wantRedact == "" && deny {
				t.Fatalf("deny inesperado, payload=%s", c.payload)
			}
			switch c.wantRedact {
			case "total":
				if res.Action != "redact-total" {
					t.Fatalf("esperava redact-total, got %q", res.Action)
				}
			case "inline":
				if res.Action != "redact-inline" {
					t.Fatalf("esperava redact-inline, got %q", res.Action)
				}
			}
			if res.Action == "redact-inline" && strings.Contains(res.Text, c.payload) {
				t.Fatal("texto redigido ainda contem segredo")
			}
		})
	}
}

func TestGuardInlineLiterals(t *testing.T) {
	readLit := func(name string) string {
		b, err := os.ReadFile("testdata/" + name + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	jwt, aws, ghp := readLit("jwt"), readLit("aws"), readLit("ghp")

	if deny, reason := Check([]byte(`{"tool_name":"Bash","tool_input":{"command":"curl -H Authorization: Bearer ` + jwt + `"}}`)); !deny {
		t.Fatal("esperava deny para JWT inline")
	} else if !strings.Contains(reason, "JWT") {
		t.Fatalf("reason deveria citar JWT, got %q", reason)
	}

	if deny, reason := Check([]byte(`{"tool_name":"Bash","tool_input":{"command":"aws s3 ls --access-key ` + aws + `"}}`)); !deny {
		t.Fatal("esperava deny para AWS key inline")
	} else if !strings.Contains(reason, "AWS_ACCESS_KEY") {
		t.Fatalf("reason deveria citar AWS_ACCESS_KEY, got %q", reason)
	}

	res := Redact([]byte(`{"tool_output":"token: ` + jwt + `"}`))
	if res.Action != "redact-inline" {
		t.Fatalf("esperava redact-inline, got %q", res.Action)
	}
	if strings.Contains(res.Text, jwt) {
		t.Fatal("texto redigido ainda contem JWT")
	}

	res = Redact([]byte(`{"tool_output":"export TOKEN=` + ghp + `"}`))
	if res.Action != "redact-inline" {
		t.Fatalf("esperava redact-inline para PAT, got %q", res.Action)
	}
	if strings.Contains(res.Text, ghp) {
		t.Fatal("texto redigido ainda contem PAT")
	}

	res = Redact([]byte(`{"tool_name":"Read","tool_input":{"file_path":"/proj/main.go"},"tool_output":"const T = ` + jwt + `"}`))
	if res.Action != "redact-inline" {
		t.Fatalf("esperava redact-inline em arquivo legit, got %q", res.Action)
	}
}

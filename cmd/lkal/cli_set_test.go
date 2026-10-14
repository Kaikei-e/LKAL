package main

import (
	"os"
	"strings"
	"testing"

	lkal "github.com/Kaikei-e/LKAL"
)

func TestSet(t *testing.T) {
	t.Run("spec command updates image and keeps comments", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/image", `"ghcr.io/acme/web:1.5.0"`})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("set:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		wantContent := strings.Replace(specExampleDoc, "1.4.2", "1.5.0", 1)
		if string(data) != wantContent {
			t.Errorf("file content mismatch:\ngot:\n%s\nwant:\n%s", string(data), wantContent)
		}
		f, parseErr := lkal.Parse(p, data)
		if parseErr != nil {
			t.Errorf("rewritten file failed to parse: %v", parseErr)
		} else if f.Version() != "0.1" {
			t.Errorf("version: got %q, want \"0.1\"", f.Version())
		}
	})

	t.Run("set adds element under collection", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/port/admin", "{ number = 8081, protocol = .tcp }"})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("set add element:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "admin") || !strings.Contains(content, "8081") {
			t.Errorf("new element missing from file: %s", content)
		}
		if _, parseErr := lkal.Parse(p, data); parseErr != nil {
			t.Errorf("rewritten file failed to parse: %v", parseErr)
		}
	})

	t.Run("bad value exits 1 and leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/replicas", "tcp"})
		wantStderr := "<value>:1:1: L102 bare word in value position\n  hint: write \"tcp\" for a string or .tcp for an enum\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set bad value:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("bad value with json flag has file <value>", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", "--json", p, "service/web/replicas", "tcp"})
		wantStderr := "[{\"file\":\"<value>\",\"line\":1,\"col\":1,\"code\":\"L102\",\"message\":\"bare word in value position\",\"hint\":\"write \\\"tcp\\\" for a string or .tcp for an enum\"}]\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set bad value --json:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("non-record value for collection element", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/port/http", "1"})
		wantStderr := p + ": L403 invalid value\n  hint: use a record for a collection element\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set non-record on collection element:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("malformed path", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "bad//path", "\"val\""})
		wantStderr := p + ": L401 malformed path\n  hint: separate segments with / and quote segments that contain other characters\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set malformed path:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("missing parent exits 1 and leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "web.lkal", specExampleDoc)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/missing/image", "\"foo\""})
		wantStderr := p + ": L402 path not found\n  hint: check the path at segment \"missing\"\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("set missing parent:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != specExampleDoc {
			t.Errorf("file was modified on error")
		}
	})

	t.Run("write failure on read-only file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("skipping write failure test as root")
		}
		dir := t.TempDir()
		p := writeTempMode(t, dir, "ro.lkal", specExampleDoc, 0400)

		code, stdout, stderr := execRun(t, []string{"set", p, "service/web/container/app/image", `"ghcr.io/acme/web:1.5.0"`})
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "lkal: ") {
			t.Errorf("set on read-only file:\nexit code: got %d, want 2\nstdout: got %q, want \"\"\nstderr: got %q, want prefix \"lkal: \"",
				code, stdout, stderr)
		}
	})
}

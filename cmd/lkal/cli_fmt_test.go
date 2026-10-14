package main

import (
	"fmt"
	"os"
	"testing"
)

func TestFmt(t *testing.T) {
	t.Run("single non-canonical to stdout", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		p := writeTemp(t, dir, "doc.lkal", orig)

		code, stdout, stderr := execRun(t, []string{"fmt", p})
		wantStdout := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != orig {
			t.Errorf("file modified: got %q, want %q", string(data), orig)
		}
	})

	t.Run("fmt with json flag on success", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		p := writeTemp(t, dir, "doc.lkal", orig)

		code, stdout, stderr := execRun(t, []string{"fmt", "--json", p})
		wantStdout := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt --json:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
	})

	t.Run("multiple files in argument order", func(t *testing.T) {
		dir := t.TempDir()
		p1 := writeTemp(t, dir, "doc1.lkal", "@lkal \"0.1\"\na = 1\n")
		p2 := writeTemp(t, dir, "doc2.lkal", "@lkal \"0.1\"\nb = 2\n")

		code, stdout, stderr := execRun(t, []string{"fmt", p1, p2})
		wantStdout := "@lkal \"0.1\"\n\na = 1\n@lkal \"0.1\"\n\nb = 2\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt multiple:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
	})

	t.Run("l flag lists only non-canonical", func(t *testing.T) {
		dir := t.TempDir()
		p1 := writeTemp(t, dir, "dirty.lkal", "@lkal \"0.1\"\na = 1, b = 2\n")
		p2 := writeTemp(t, dir, "clean.lkal", "@lkal \"0.1\"\n\nc = 3\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", p1, p2})
		wantStdout := p1 + "\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt -l:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
	})

	t.Run("l flag all canonical", func(t *testing.T) {
		dir := t.TempDir()
		p := writeTemp(t, dir, "clean.lkal", "@lkal \"0.1\"\n\na = 1\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", p})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("fmt -l clean:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
	})

	t.Run("w flag rewrites non-canonical and preserves mode", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\na = 1, b = [1,2]\n"
		p := writeTempMode(t, dir, "doc.lkal", orig, 0640)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", p})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("fmt -w:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		wantData := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		if err != nil || string(data) != wantData {
			t.Errorf("file data: got %q, want %q", string(data), wantData)
		}
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if fi.Mode().Perm() != 0640 {
			t.Errorf("mode: got %o, want 0640", fi.Mode().Perm())
		}
	})

	t.Run("w flag leaves canonical untouched", func(t *testing.T) {
		dir := t.TempDir()
		orig := "@lkal \"0.1\"\n\na = 1\n"
		p := writeTempMode(t, dir, "doc.lkal", orig, 0644)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", p})
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("fmt -w canonical:\nexit code: got %d, want 0\nstdout: got %q, want \"\"\nstderr: got %q, want \"\"",
				code, stdout, stderr)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != orig {
			t.Errorf("file data: got %q, want %q", string(data), orig)
		}
	})

	t.Run("l and w flags together", func(t *testing.T) {
		dir := t.TempDir()
		p1 := writeTemp(t, dir, "dirty.lkal", "@lkal \"0.1\"\na = 1, b = 2\n")
		p2 := writeTemp(t, dir, "clean.lkal", "@lkal \"0.1\"\n\nc = 3\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", "-w", p1, p2})
		wantStdout := p1 + "\n"
		if code != 0 || stdout != wantStdout || stderr != "" {
			t.Errorf("fmt -l -w:\nexit code: got %d, want 0\nstdout: got %q, want %q\nstderr: got %q, want \"\"",
				code, stdout, wantStdout, stderr)
		}
		data1, err := os.ReadFile(p1)
		wantData1 := "@lkal \"0.1\"\n\na = 1\nb = 2\n"
		if err != nil || string(data1) != wantData1 {
			t.Errorf("dirty file rewritten: got %q, want %q", string(data1), wantData1)
		}
	})

	t.Run("plain fmt with mix of valid and invalid files", func(t *testing.T) {
		dir := t.TempDir()
		pGood := writeTemp(t, dir, "good.lkal", "@lkal \"0.1\"\na = 1, b = [1,2]\n")
		pBad := writeTemp(t, dir, "bad.lkal", "@lkal \"0.1\"\na = b\n")

		code, stdout, stderr := execRun(t, []string{"fmt", pGood, pBad})
		wantStdout := "@lkal \"0.1\"\n\na = 1\nb = [1, 2]\n"
		wantStderr := pBad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
		if code != 1 || stdout != wantStdout || stderr != wantStderr {
			t.Errorf("fmt good bad:\nexit code: got %d, want 1\nstdout: got %q, want %q\nstderr: got %q, want %q",
				code, stdout, wantStdout, stderr, wantStderr)
		}
	})

	t.Run("l flag with mix of valid and invalid files", func(t *testing.T) {
		dir := t.TempDir()
		pGood := writeTemp(t, dir, "good.lkal", "@lkal \"0.1\"\na = 1, b = [1,2]\n")
		pBad := writeTemp(t, dir, "bad.lkal", "@lkal \"0.1\"\na = b\n")

		code, stdout, stderr := execRun(t, []string{"fmt", "-l", pGood, pBad})
		wantStdout := pGood + "\n"
		wantStderr := pBad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
		if code != 1 || stdout != wantStdout || stderr != wantStderr {
			t.Errorf("fmt -l good bad:\nexit code: got %d, want 1\nstdout: got %q, want %q\nstderr: got %q, want %q",
				code, stdout, wantStdout, stderr, wantStderr)
		}
	})

	t.Run("w flag isolation with error", func(t *testing.T) {
		dir := t.TempDir()
		badOrig := "@lkal \"0.1\"\na = b\n"
		goodOrig := "@lkal \"0.1\"\nx = 1, y = 2\n"
		pBad := writeTemp(t, dir, "bad.lkal", badOrig)
		pGood := writeTemp(t, dir, "good.lkal", goodOrig)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", pBad, pGood})
		wantStderr := pBad + ":2:5: L102 bare word in value position\n  hint: write \"b\" for a string or .b for an enum\n"
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("fmt -w with error:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		badData, err := os.ReadFile(pBad)
		if err != nil || string(badData) != badOrig {
			t.Errorf("bad file was modified: got %q, want %q", string(badData), badOrig)
		}
		goodData, err := os.ReadFile(pGood)
		wantGood := "@lkal \"0.1\"\n\nx = 1\ny = 2\n"
		if err != nil || string(goodData) != wantGood {
			t.Errorf("good file was not rewritten: got %q, want %q", string(goodData), wantGood)
		}
	})

	t.Run("w flag with json flag and error", func(t *testing.T) {
		dir := t.TempDir()
		badOrig := "@lkal \"0.1\"\na = b\n"
		goodOrig := "@lkal \"0.1\"\nx = 1, y = 2\n"
		pBad := writeTemp(t, dir, "bad.lkal", badOrig)
		pGood := writeTemp(t, dir, "good.lkal", goodOrig)

		code, stdout, stderr := execRun(t, []string{"fmt", "-w", "--json", pBad, pGood})
		wantStderr := fmt.Sprintf(`[{"file":%q,"line":2,"col":5,"code":"L102","message":"bare word in value position","hint":"write \"b\" for a string or .b for an enum"}]`+"\n", pBad)
		if code != 1 || stdout != "" || stderr != wantStderr {
			t.Errorf("fmt -w --json with error:\nexit code: got %d, want 1\nstdout: got %q, want \"\"\nstderr: got %q, want %q",
				code, stdout, stderr, wantStderr)
		}
		goodData, err := os.ReadFile(pGood)
		wantGood := "@lkal \"0.1\"\n\nx = 1\ny = 2\n"
		if err != nil || string(goodData) != wantGood {
			t.Errorf("good file was not rewritten: got %q, want %q", string(goodData), wantGood)
		}
	})
}

package main

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"flag"
	"fmt"
	"io"
	"os"

	lkal "github.com/Kaikei-e/LKAL"
)

const usageText = "usage: lkal <command> [flags] [args...]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "fmt":
			return cmdFmt(args[1:], stdout, stderr)
		case "check":
			return cmdCheck(args[1:], stderr)
		case "json":
			return cmdJSON(args[1:], stdout, stderr)
		case "from-json":
			return cmdFromJSON(args[1:], stdout, stderr)
		case "get":
			return cmdGet(args[1:], stdout, stderr)
		case "set":
			return cmdSet(args[1:], stderr)
		case "del":
			return cmdDel(args[1:], stderr)
		}
	}
	fmt.Fprintln(stderr, usageText)
	return 2
}

type jsonErrorEntry struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func reportErrors(errs []*lkal.Error, jsonOutput bool, stderr io.Writer) {
	if len(errs) == 0 {
		return
	}
	if jsonOutput {
		entries := make([]jsonErrorEntry, len(errs))
		for i, e := range errs {
			entries[i] = jsonErrorEntry{
				File:    e.File,
				Line:    e.Pos.Line,
				Col:     e.Pos.Col,
				Code:    e.Code,
				Message: e.Msg,
				Hint:    e.Hint,
			}
		}
		if err := json.MarshalWrite(stderr, entries, jsontext.AllowInvalidUTF8(true)); err != nil {
			fmt.Fprintln(stderr, "lkal:", err)
			return
		}
		fmt.Fprintln(stderr)
		return
	}
	for _, e := range errs {
		fmt.Fprintln(stderr, e)
	}
}

func toErrors(err error) []*lkal.Error {
	return err.(*lkal.ErrorList).Errors
}

func parseFlags(name, usage string, args []string, stderr io.Writer, minArgs, maxArgs int, extra func(*flag.FlagSet)) (bool, []string, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	fs.Usage = func() { fmt.Fprintln(stderr, "usage: lkal "+usage) }
	if extra != nil {
		extra(fs)
	}
	if fs.Parse(args) != nil {
		return false, nil, false
	}
	a := fs.Args()
	if len(a) < minArgs || (maxArgs >= 0 && len(a) > maxArgs) {
		fs.Usage()
		return false, nil, false
	}
	return *jsonOut, a, true
}

func readAndParse(file string, jsonOut bool, stderr io.Writer) (*lkal.File, int) {
	src, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(stderr, "lkal:", err)
		return nil, 2
	}
	f, parseErr := lkal.Parse(file, src)
	if parseErr != nil {
		reportErrors(toErrors(parseErr), jsonOut, stderr)
		return nil, 1
	}
	return f, 0
}

func cmdCheck(args []string, stderr io.Writer) int {
	jsonOut, files, ok := parseFlags("check", "check [--json] FILE...", args, stderr, 1, -1, nil)
	if !ok {
		return 2
	}
	var (
		allErrors []*lkal.Error
		hasIO     bool
	)
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(stderr, "lkal:", err)
			hasIO = true
			continue
		}
		if _, parseErr := lkal.Parse(file, src); parseErr != nil {
			allErrors = append(allErrors, toErrors(parseErr)...)
		}
	}
	reportErrors(allErrors, jsonOut, stderr)
	if hasIO {
		return 2
	}
	if len(allErrors) > 0 {
		return 1
	}
	return 0
}

func cmdFmt(args []string, stdout, stderr io.Writer) int {
	var w, l bool
	jsonOut, files, ok := parseFlags("fmt", "fmt [-w] [-l] [--json] FILE...", args, stderr, 1, -1, func(fs *flag.FlagSet) {
		fs.BoolVar(&w, "w", false, "")
		fs.BoolVar(&l, "l", false, "")
	})
	if !ok {
		return 2
	}
	var (
		allErrors []*lkal.Error
		hasIO     bool
	)
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(stderr, "lkal:", err)
			hasIO = true
			continue
		}
		f, parseErr := lkal.Parse(file, src)
		if parseErr != nil {
			allErrors = append(allErrors, toErrors(parseErr)...)
			continue
		}
		canonical := f.Format()
		differs := !bytes.Equal(src, canonical)
		switch {
		case !w && !l:
			stdout.Write(canonical)
		case !differs:
		case w && l:
			fmt.Fprintln(stdout, file)
			if err := os.WriteFile(file, canonical, 0666); err != nil {
				fmt.Fprintln(stderr, "lkal:", err)
				hasIO = true
			}
		case l:
			fmt.Fprintln(stdout, file)
		case w:
			if err := os.WriteFile(file, canonical, 0666); err != nil {
				fmt.Fprintln(stderr, "lkal:", err)
				hasIO = true
			}
		}
	}
	reportErrors(allErrors, jsonOut, stderr)
	if hasIO {
		return 2
	}
	if len(allErrors) > 0 {
		return 1
	}
	return 0
}

func cmdJSON(args []string, stdout, stderr io.Writer) int {
	var plain bool
	jsonOut, files, ok := parseFlags("json", "json [-plain] [--json] FILE", args, stderr, 1, 1, func(fs *flag.FlagSet) {
		fs.BoolVar(&plain, "plain", false, "")
	})
	if !ok {
		return 2
	}
	f, code := readAndParse(files[0], jsonOut, stderr)
	if code != 0 {
		return code
	}
	out, _ := lkal.ToJSON(f, plain)
	stdout.Write(out)
	return 0
}

func cmdFromJSON(args []string, stdout, stderr io.Writer) int {
	jsonOut, files, ok := parseFlags("from-json", "from-json [--json] FILE", args, stderr, 1, 1, nil)
	if !ok {
		return 2
	}
	src, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintln(stderr, "lkal:", err)
		return 2
	}
	f, err := lkal.FromJSON(src)
	if err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = files[0]
		}
		reportErrors(errs, jsonOut, stderr)
		return 1
	}
	stdout.Write(f.Format())
	return 0
}

func cmdGet(args []string, stdout, stderr io.Writer) int {
	jsonOut, pos, ok := parseFlags("get", "get [--json] FILE PATH", args, stderr, 2, 2, nil)
	if !ok {
		return 2
	}
	f, code := readAndParse(pos[0], jsonOut, stderr)
	if code != 0 {
		return code
	}
	val, err := f.Get(pos[1])
	if err != nil {
		reportErrors(toErrors(err), jsonOut, stderr)
		return 1
	}
	stdout.Write(append(lkal.FormatValue(val), '\n'))
	return 0
}

func cmdSet(args []string, stderr io.Writer) int {
	jsonOut, pos, ok := parseFlags("set", "set [--json] FILE PATH VALUE", args, stderr, 3, 3, nil)
	if !ok {
		return 2
	}
	val, err := lkal.ParseValue([]byte(pos[2]))
	if err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = "<value>"
		}
		reportErrors(errs, jsonOut, stderr)
		return 1
	}
	f, code := readAndParse(pos[0], jsonOut, stderr)
	if code != 0 {
		return code
	}
	if err := f.Set(pos[1], val); err != nil {
		reportErrors(toErrors(err), jsonOut, stderr)
		return 1
	}
	if err := os.WriteFile(pos[0], f.Format(), 0666); err != nil {
		fmt.Fprintln(stderr, "lkal:", err)
		return 2
	}
	return 0
}

func cmdDel(args []string, stderr io.Writer) int {
	jsonOut, pos, ok := parseFlags("del", "del [--json] FILE PATH", args, stderr, 2, 2, nil)
	if !ok {
		return 2
	}
	f, code := readAndParse(pos[0], jsonOut, stderr)
	if code != 0 {
		return code
	}
	if err := f.Delete(pos[1]); err != nil {
		reportErrors(toErrors(err), jsonOut, stderr)
		return 1
	}
	if err := os.WriteFile(pos[0], f.Format(), 0666); err != nil {
		fmt.Fprintln(stderr, "lkal:", err)
		return 2
	}
	return 0
}

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	lkal "github.com/Kaikei-e/LKAL"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "fmt":
		return cmdFmt(args[1:], stdout, stderr)
	case "check":
		return cmdCheck(args[1:], stdout, stderr)
	case "json":
		return cmdJSON(args[1:], stdout, stderr)
	case "from-json":
		return cmdFromJSON(args[1:], stdout, stderr)
	case "get":
		return cmdGet(args[1:], stdout, stderr)
	case "set":
		return cmdSet(args[1:], stdout, stderr)
	case "del":
		return cmdDel(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: lkal <command> [flags] [args...]")
}

func ioError(w io.Writer, err error) int {
	fmt.Fprintf(w, "lkal: %v\n", err)
	return 2
}

func writeFileKeepPerm(name string, data []byte) error {
	fi, err := os.Stat(name)
	if err != nil {
		return err
	}
	return os.WriteFile(name, data, fi.Mode().Perm())
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
		enc := json.NewEncoder(stderr)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(entries)
		return
	}
	for _, e := range errs {
		fmt.Fprintf(stderr, "%s\n", e.Error())
	}
}

func toErrors(err error) []*lkal.Error {
	if err == nil {
		return nil
	}
	if el, ok := err.(*lkal.ErrorList); ok {
		return el.Errors
	}
	if e, ok := err.(*lkal.Error); ok {
		return []*lkal.Error{e}
	}
	return nil
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal check [--json] FILE...")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	if len(files) < 1 {
		fs.Usage()
		return 2
	}

	var allErrors []*lkal.Error
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return ioError(stderr, err)
		}
		_, parseErr := lkal.Parse(file, src)
		if parseErr != nil {
			allErrors = append(allErrors, toErrors(parseErr)...)
		}
	}
	if len(allErrors) > 0 {
		reportErrors(allErrors, *jsonFlag, stderr)
		return 1
	}
	return 0
}

func cmdFmt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	wFlag := fs.Bool("w", false, "")
	lFlag := fs.Bool("l", false, "")
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal fmt [-w] [-l] [--json] FILE...")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	if len(files) < 1 {
		fs.Usage()
		return 2
	}

	var allErrors []*lkal.Error
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return ioError(stderr, err)
		}
		f, parseErr := lkal.Parse(file, src)
		if parseErr != nil {
			allErrors = append(allErrors, toErrors(parseErr)...)
			continue
		}
		canonical := f.Format()
		differs := !bytes.Equal(src, canonical)

		if *wFlag || *lFlag {
			if *lFlag && differs {
				fmt.Fprintln(stdout, file)
			}
			if *wFlag && differs {
				if err := writeFileKeepPerm(file, canonical); err != nil {
					return ioError(stderr, err)
				}
			}
		} else {
			stdout.Write(canonical)
		}
	}
	if len(allErrors) > 0 {
		reportErrors(allErrors, *jsonFlag, stderr)
		return 1
	}
	return 0
}

func cmdJSON(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("json", flag.ContinueOnError)
	fs.SetOutput(stderr)
	plainFlag := fs.Bool("plain", false, "")
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal json [-plain] [--json] FILE")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	if len(files) != 1 {
		fs.Usage()
		return 2
	}
	file := files[0]
	src, err := os.ReadFile(file)
	if err != nil {
		return ioError(stderr, err)
	}
	f, parseErr := lkal.Parse(file, src)
	if parseErr != nil {
		reportErrors(toErrors(parseErr), *jsonFlag, stderr)
		return 1
	}
	out, err := lkal.ToJSON(f, *plainFlag)
	if err != nil {
		if errs := toErrors(err); len(errs) > 0 {
			reportErrors(errs, *jsonFlag, stderr)
			return 1
		}
		fmt.Fprintf(stderr, "lkal: %v\n", err)
		return 1
	}
	stdout.Write(out)
	return 0
}

func cmdFromJSON(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("from-json", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal from-json [--json] FILE")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	if len(files) != 1 {
		fs.Usage()
		return 2
	}
	file := files[0]
	src, err := os.ReadFile(file)
	if err != nil {
		return ioError(stderr, err)
	}
	f, err := lkal.FromJSON(src)
	if err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = file
		}
		reportErrors(errs, *jsonFlag, stderr)
		return 1
	}
	stdout.Write(f.Format())
	return 0
}

func cmdGet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal get [--json] FILE PATH")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	posArgs := fs.Args()
	if len(posArgs) != 2 {
		fs.Usage()
		return 2
	}
	file, path := posArgs[0], posArgs[1]
	src, err := os.ReadFile(file)
	if err != nil {
		return ioError(stderr, err)
	}
	f, parseErr := lkal.Parse(file, src)
	if parseErr != nil {
		reportErrors(toErrors(parseErr), *jsonFlag, stderr)
		return 1
	}
	val, err := f.Get(path)
	if err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = file
		}
		reportErrors(errs, *jsonFlag, stderr)
		return 1
	}
	out := lkal.FormatValue(val)
	stdout.Write(append(out, '\n'))
	return 0
}

func cmdSet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal set [--json] FILE PATH VALUE")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	posArgs := fs.Args()
	if len(posArgs) != 3 {
		fs.Usage()
		return 2
	}
	file, path, value := posArgs[0], posArgs[1], posArgs[2]

	src, err := os.ReadFile(file)
	if err != nil {
		return ioError(stderr, err)
	}

	val, err := lkal.ParseValue([]byte(value))
	if err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = "<value>"
		}
		reportErrors(errs, *jsonFlag, stderr)
		return 1
	}

	f, parseErr := lkal.Parse(file, src)
	if parseErr != nil {
		reportErrors(toErrors(parseErr), *jsonFlag, stderr)
		return 1
	}

	if err := f.Set(path, val); err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = file
		}
		reportErrors(errs, *jsonFlag, stderr)
		return 1
	}

	if err := writeFileKeepPerm(file, f.Format()); err != nil {
		return ioError(stderr, err)
	}
	return 0
}

func cmdDel(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("del", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonFlag := fs.Bool("json", false, "")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lkal del [--json] FILE PATH")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	posArgs := fs.Args()
	if len(posArgs) != 2 {
		fs.Usage()
		return 2
	}
	file, path := posArgs[0], posArgs[1]

	src, err := os.ReadFile(file)
	if err != nil {
		return ioError(stderr, err)
	}

	f, parseErr := lkal.Parse(file, src)
	if parseErr != nil {
		reportErrors(toErrors(parseErr), *jsonFlag, stderr)
		return 1
	}

	if err := f.Delete(path); err != nil {
		errs := toErrors(err)
		for _, e := range errs {
			e.File = file
		}
		reportErrors(errs, *jsonFlag, stderr)
		return 1
	}

	if err := writeFileKeepPerm(file, f.Format()); err != nil {
		return ioError(stderr, err)
	}
	return 0
}

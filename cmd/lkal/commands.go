package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	lkal "github.com/Kaikei-e/LKAL"
)

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
			_, _ = stdout.Write(canonical)
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
	_, _ = stdout.Write(out)
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
		reportErrors(withFileName(toErrors(err), files[0]), jsonOut, stderr)
		return 1
	}
	_, _ = stdout.Write(f.Format())
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
	_, _ = stdout.Write(append(lkal.FormatValue(val), '\n'))
	return 0
}

func cmdSet(args []string, stderr io.Writer) int {
	jsonOut, pos, ok := parseFlags("set", "set [--json] FILE PATH VALUE", args, stderr, 3, 3, nil)
	if !ok {
		return 2
	}
	val, err := lkal.ParseValue([]byte(pos[2]))
	if err != nil {
		reportErrors(withFileName(toErrors(err), "<value>"), jsonOut, stderr)
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

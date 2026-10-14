package main

import (
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

func run(args []string, _ io.Reader, stdout, stderr io.Writer) int {
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

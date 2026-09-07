package command

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

var output io.Writer = os.Stderr

var (
	addMark  = color.New(color.FgGreen).Sprint("+")
	keepMark = color.New(color.FgYellow).Sprint("=")
	warnMark = color.New(color.FgYellow).Sprint("!")
	missMark = color.New(color.FgRed).Sprint("-")
)

func ReportMissing(paths ...string) {
	for _, path := range paths {
		_, _ = fmt.Fprintf(output, "  %s %s — missing\n", missMark, path)
	}
}

// Not an error: the run succeeded, and this is about something it deliberately
// did not touch.
func ReportWarning(text string) {
	_, _ = fmt.Fprintf(output, "  %s %s\n", warnMark, text)
}

func reportCreated(paths []string) {
	for _, path := range paths {
		_, _ = fmt.Fprintf(output, "  %s %s\n", addMark, path)
	}
}

// Scaffolding never overwrites, so a second run looks like one that did nothing.
// Saying what was left alone is how a changed template stops being invisible.
func reportKept(paths []string) {
	for _, path := range paths {
		_, _ = fmt.Fprintf(output, "  %s %s — exists, left alone\n", keepMark, path)
	}
}

// For a caller that decides for itself: a migration carries a timestamp, so the
// name it would get on this run recognises nothing.
func ReportKept(paths ...string) {
	reportKept(paths)
}

func reportAction(text string) {
	_, _ = fmt.Fprintf(output, "  %s %s\n", addMark, text)
}

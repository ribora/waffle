package command

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Scaffolding never overwrites, so a second run looks like one that did nothing.
// Saying what was left alone is how a changed template stops being invisible.
var output io.Writer = os.Stderr

var (
	addMark    = color.New(color.FgGreen).Sprint("+")
	keepMark   = color.New(color.FgYellow).Sprint("=")
	removeMark = color.New(color.FgRed).Sprint("-")
	warnMark   = color.New(color.FgYellow).Sprint("!")
)

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

// For a caller that writes its own files rather than going through Write — the
// mock generator shells out, and mockgen creates the destination itself.
func ReportCreated(paths ...string) {
	reportCreated(paths)
}

// Loud on purpose: a run that silently shrinks the tree is worse than one that
// grows it. The reason is the caller's — this package is shared.
func ReportRemoved(reason string, paths ...string) {
	for _, path := range paths {
		_, _ = fmt.Fprintf(output, "  %s %s — %s\n", removeMark, path, reason)
	}
}

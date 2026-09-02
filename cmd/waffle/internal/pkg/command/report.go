package command

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Scaffolding stays idempotent by never overwriting, which makes a second run look
// exactly like a run that did nothing. So it says what it left alone: otherwise a
// changed template silently leaves half a domain on the previous version.
var output io.Writer = os.Stderr

var (
	addMark  = color.New(color.FgGreen).Sprint("+")
	keepMark = color.New(color.FgYellow).Sprint("=")
)

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

// For a caller that decides for itself that a target is already there: a migration
// carries a timestamp in its name and cannot be recognised by the name it would get
// on this run.
func ReportKept(paths ...string) {
	reportKept(paths)
}

func reportAction(text string) {
	_, _ = fmt.Fprintf(output, "  %s %s\n", addMark, text)
}

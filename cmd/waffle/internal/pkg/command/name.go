package command

import (
	"errors"
	"fmt"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// Lowercase-first because the templates use the name both unexported and,
// through `title`, exported.
var nameShape = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Refused before a file is written: the name reaches the templates as a Go
// identifier.
func ValidateName(kind, name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%s is empty", kind)
	case !nameShape.MatchString(name):
		return fmt.Errorf("%s %q must be a lowercase Go identifier: a letter, then letters, digits or underscores", kind, name)
	case token.IsKeyword(name):
		return fmt.Errorf("%s %q is a Go keyword", kind, name)
	}
	return nil
}

// One name, which a CLI token spells differently from a Go package.
type CommandName struct {
	Raw   string // sweep-otp — the CLI token and the directory
	Snake string // sweep_otp — the DI registration's file name
	Pkg   string // sweepotp — a Go package name carries no separator
	Lower string // sweepOtp — the unexported identifier
	Upper string // SweepOtp — the exported identifier and the import alias
	Words string // sweep otp — the generated description
}

// Separators only between words, so two spellings cannot derive one package.
var commandShape = regexp.MustCompile(`^[a-z][a-z0-9]*([-_][a-z0-9]+)*$`)

func ParseCommandName(kind, name string) (CommandName, error) {
	switch {
	case name == "":
		return CommandName{}, fmt.Errorf("%s is empty", kind)
	case !commandShape.MatchString(name):
		return CommandName{}, fmt.Errorf("%s %q must be lowercase words joined by - or _: a letter, then letters or digits", kind, name)
	}

	words := strings.FieldsFunc(name, separator)
	out := CommandName{
		Raw:   name,
		Snake: strings.Join(words, "_"),
		Pkg:   strings.Join(words, ""),
		Words: strings.Join(words, " "),
		Lower: Camel(name),
		Upper: Title(name),
	}

	// A keyword is one word, so Pkg and Lower are the same check.
	if token.IsKeyword(out.Pkg) {
		return CommandName{}, fmt.Errorf("%s %q is a Go keyword", kind, name)
	}
	return out, nil
}

// Folding, not just uppercasing: ValidateName accepts order_item.
func Title(name string) string {
	return upper(Camel(name))
}

func Camel(name string) string {
	words := strings.FieldsFunc(name, separator)
	if len(words) == 0 {
		return ""
	}
	out := words[0]
	for _, word := range words[1:] {
		out += upper(word)
	}
	return out
}

func upper(word string) string {
	if word == "" {
		return ""
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

func separator(r rune) bool { return r == '-' || r == '_' }

// Called once a command has taken the flags it knows, so an unrecognised flag
// fails instead of becoming a positional.
func RejectFlags(args []string) error {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return fmt.Errorf("unknown flag %q", a)
		}
	}
	return nil
}

// An extra operand used to be dropped in silence: `make:domain:migration orders
// add index` wrote a migration named "add". Takes RejectFlags with it, so a
// command cannot close one gap and leave the other.
func (c *Command) Operands(args []string, min, max int) error {
	if err := RejectFlags(args); err != nil {
		return fmt.Errorf("%w\n%s", err, c.Usage())
	}
	if len(args) < min {
		return errors.New(c.Usage())
	}
	if len(args) > max {
		return fmt.Errorf("unexpected operand %q\n%s", args[max], c.Usage())
	}
	return nil
}

// For an operand that becomes one path segment, not a path.
func ValidateFileName(kind, name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%s is empty", kind)
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("%s %q must be a single name, not a path", kind, name)
	case name == "." || name == "..":
		return fmt.Errorf("%s %q is not a name", kind, name)
	}
	return nil
}

// For an operand that becomes a directory to write into: the generators resolve
// it against the project root.
func ValidatePath(kind, p string) error {
	switch {
	case p == "":
		return fmt.Errorf("%s is empty", kind)
	case filepath.IsAbs(p):
		return fmt.Errorf("%s %q must be relative to the project root", kind, p)
	}
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		if seg == ".." {
			return fmt.Errorf("%s %q must stay inside the project", kind, p)
		}
	}
	return nil
}

// Deliberately thin: the broker owns the grammar, this only rejects literals
// that would not survive being written into Keys().
func ValidateRoutingKey(key string) error {
	switch {
	case key == "":
		return fmt.Errorf("routing key is empty")
	case strings.ContainsAny(key, " \t\n\""):
		return fmt.Errorf("routing key %q contains whitespace or a quote", key)
	}
	return nil
}

func IsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

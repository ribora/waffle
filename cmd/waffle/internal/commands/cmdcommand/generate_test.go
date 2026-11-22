package cmdcommand

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command/commandtest"
)

const project = "example.com/svc"

// Which spelling landed where — format.Source already proved it parses.
func TestEachRoleGetsItsOwnSpelling(t *testing.T) {
	main, di := generate(t, "media", "sweep-otp")

	for _, want := range []string{
		"package sweepotp",
		`app.NewCommand("media:sweep-otp", "Sweep otp media")`,
		"func NewSweepOtpCommand(",
		"type sweepOtpCommand struct",
		"func (c *sweepOtpCommand) Run(",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main.go does not contain %q:\n%s", want, main)
		}
	}
	for _, want := range []string{
		`cmdSweepOtp "example.com/svc/domain/media/cmd/sweep-otp"`,
		"func InitMediaSweepOtp()",
		"di.Define(cmdSweepOtp.NewSweepOtpCommand",
	} {
		if !strings.Contains(di, want) {
			t.Errorf("the DI registration does not contain %q:\n%s", want, di)
		}
	}
}

// A bare name collides with the framework's own migrate, and app.Init then fails.
func TestTheTokenIsNamespacedByDomain(t *testing.T) {
	main, _ := generate(t, "billing", "migrate")

	if !strings.Contains(main, `app.NewCommand("billing:migrate"`) {
		t.Errorf("the token is not namespaced, so it collides with the migrate daemon:\n%s", main)
	}
}

// An upgrade must not rename the commands a project already has.
func TestASingleWordNameIsUnchangedApartFromTheToken(t *testing.T) {
	main, di := generate(t, "rent", "recalc")

	for _, want := range []string{"package recalc", "func NewRecalcCommand(", "type recalcCommand struct"} {
		if !strings.Contains(main, want) {
			t.Errorf("main.go does not contain %q:\n%s", want, main)
		}
	}
	if !strings.Contains(di, "func InitRentRecalc()") {
		t.Errorf("the DI registration does not contain InitRentRecalc:\n%s", di)
	}
}

// How a command is spelled on a command line is the author's, so both separators
// reach the token and the directory as they were typed.
func TestTheSeparatorTypedIsTheSeparatorRegistered(t *testing.T) {
	underscore, _ := generate(t, "media", "sweep_otp")
	if !strings.Contains(underscore, `app.NewCommand("media:sweep_otp"`) {
		t.Errorf("the token does not keep the separator:\n%s", underscore)
	}

	hyphen, _ := generate(t, "media", "sweep-otp")
	for _, same := range []string{"package sweepotp", "func NewSweepOtpCommand(", "type sweepOtpCommand struct"} {
		if !strings.Contains(underscore, same) || !strings.Contains(hyphen, same) {
			t.Errorf("%q does not come out of both spellings", same)
		}
	}
}

// The directory keeps the hyphen, the Go file cannot.
func TestTheWrittenPaths(t *testing.T) {
	p := commandtest.Project(t, project)
	name, err := command.ParseCommandName("command", "sweep-otp")
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(p, "media", name); err != nil {
		t.Fatal(err)
	}

	var written []string
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || path == "go.mod" {
			return err
		}
		written = append(written, filepath.ToSlash(path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(written)

	want := []string{
		"config/di/di_media_sweep_otp.go",
		"domain/media/cmd/sweep-otp/main.go",
	}
	if !slices.Equal(written, want) {
		t.Errorf("written = %v, want %v", written, want)
	}
}

func generate(t *testing.T, domain, cmdName string) (main, di string) {
	t.Helper()
	p := commandtest.Project(t, project)

	name, err := command.ParseCommandName("command", cmdName)
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(p, domain, name); err != nil {
		t.Fatal(err)
	}
	return read(t, filepath.Join("domain", domain, "cmd", name.Raw, "main.go")),
		read(t, filepath.Join("config", "di", "di_"+domain+"_"+name.Snake+".go"))
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

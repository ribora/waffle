package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A monorepo with two services, and a waffle stand-in that answers as the test
// says and records what it was asked.
type fixture struct {
	root, svc, other string
	h                hook
	calls            []string
}

func newFixture(t *testing.T, answer func(args []string) (string, error)) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{root: resolve(root)}
	for _, name := range []string{"kostej", "shark"} {
		dir := filepath.Join(root, "services", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "waffle.yaml"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f.svc = resolve(filepath.Join(root, "services", "kostej"))
	f.other = resolve(filepath.Join(root, "services", "shark"))
	f.h = hook{data: t.TempDir(), run: func(dir string, args ...string) (string, error) {
		f.calls = append(f.calls, dir+" "+strings.Join(args, " "))
		return answer(args)
	}}
	return f
}

func allowing(args []string) (string, error) { return "brief of " + args[1], nil }

func (f *fixture) prompt(text string) string {
	return f.h.prompt(input{SessionID: "s1", Cwd: f.root, Prompt: text})
}

func (f *fixture) edit(path string) *decision {
	in := input{SessionID: "s1", Cwd: f.root}
	in.ToolInput.FilePath = path
	return f.h.edit(in)
}

func (f *fixture) last() string { return f.calls[len(f.calls)-1] }

func TestPromptSwitchesTheServiceNamedByItsPath(t *testing.T) {
	f := newFixture(t, allowing)
	out := f.prompt("/waffle:spec ./services/kostej PROJ-1 fix the layout")

	st := f.h.load("s1")
	if st.Mode != "spec" || st.modeOf(f.svc) != "spec" || st.modeOf(f.other) != none {
		t.Errorf("state %+v", st)
	}
	if !strings.Contains(out, "switched to spec mode, for services/kostej") || !strings.Contains(out, "brief of spec") {
		t.Errorf("context:\n%s", out)
	}
}

// waffle does not know how a repository lays its services out, so a bare name
// is the task's text even when a service directory carries it.
func TestPromptTakesAServiceOnlyByItsPath(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:spec kostej")
	if st := f.h.load("s1"); st.Mode != "" {
		t.Errorf("a bare name switched %+v", st)
	}
}

func TestPromptLeavesOtherTextAlone(t *testing.T) {
	f := newFixture(t, allowing)
	if out := f.prompt("please fix waffle.yaml"); out != "" || len(f.calls) != 0 {
		t.Errorf("an ordinary prompt produced %q and %d waffle calls", out, len(f.calls))
	}
}

func TestPromptWithNoServiceSwitchesNothing(t *testing.T) {
	f := newFixture(t, allowing)
	out := f.prompt("/waffle:spec")
	if !strings.Contains(out, "Nothing was switched") || f.h.load("s1").Mode != "" {
		t.Errorf("context %q, state %+v", out, f.h.load("s1"))
	}
}

func TestACommandWithoutAPathActsOnTheServicesNamedBefore(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:spec ./services/kostej ./services/shark")
	f.prompt("/waffle:none")
	f.prompt("/waffle:steps")
	st := f.h.load("s1")
	if st.modeOf(f.svc) != "steps" || st.modeOf(f.other) != "steps" {
		t.Errorf("state %+v", st)
	}
}

func TestNamingFewerServicesResetsTheRest(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:spec ./services/kostej ./services/shark")
	f.prompt("/waffle:steps ./services/kostej")
	st := f.h.load("s1")
	if st.modeOf(f.svc) != "steps" || st.modeOf(f.other) != none {
		t.Errorf("state %+v", st)
	}
}

func TestEditIsCheckedInTheSessionsMode(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "agents:guard" {
			return "Error: spec mode does not allow waffle.yaml; platform mode does", errors.New("exit status 1")
		}
		return allowing(args)
	})
	f.prompt("/waffle:spec ./services/kostej")

	d := f.edit(filepath.Join(f.svc, "waffle.yaml"))

	if d == nil || d.HookSpecificOutput == nil || d.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("decision = %+v", d)
	}
	if reason := d.HookSpecificOutput.PermissionDecisionReason; !strings.Contains(reason, "platform mode does") ||
		!strings.Contains(reason, "/waffle:<mode> ./services/kostej") {
		t.Errorf("reason = %q", reason)
	}
	if !strings.HasSuffix(f.last(), "agents:guard spec "+filepath.Join(f.svc, "waffle.yaml")) {
		t.Errorf("asked waffle %q", f.last())
	}
}

// While a task is on, a service it does not name is closed whole — a grant or a
// platform mode for one service reaches no other.
func TestAServiceOutsideTheTaskIsClosed(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:platform ./services/kostej")
	f.prompt("/waffle:allow model")
	calls := len(f.calls)

	d := f.edit(filepath.Join(f.other, "domain", "payment", "model", "cart.go"))

	if d == nil || d.HookSpecificOutput == nil || !strings.Contains(d.HookSpecificOutput.PermissionDecisionReason, "services/shark is not in this task") {
		t.Fatalf("decision = %+v", d)
	}
	if len(f.calls) != calls {
		t.Errorf("waffle was asked about a service outside the task: %v", f.calls[calls:])
	}
}

func TestWithNoModeEveryServiceIsCheckedAsNone(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:spec ./services/kostej")
	f.prompt("/waffle:none")
	f.edit(filepath.Join(f.other, "go.mod"))
	if want := f.other + " agents:guard none " + filepath.Join(f.other, "go.mod"); f.last() != want {
		t.Errorf("asked %q, want %q", f.last(), want)
	}
}

func TestDotDotDotNamesEveryServiceBelow(t *testing.T) {
	for _, arg := range []string{"./...", "./services/...", "services/..."} {
		t.Run(arg, func(t *testing.T) {
			f := newFixture(t, allowing)
			f.prompt("/waffle:platform " + arg)
			st := f.h.load("s1")
			if st.modeOf(f.svc) != "platform" || st.modeOf(f.other) != "platform" {
				t.Errorf("state %+v", st)
			}
		})
	}
}

// Even with a task already on, a selection that finds nothing says so rather than
// falling back to the task's services.
func TestDotDotDotBelowNoServiceSwitchesNothing(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:spec ./services/kostej")
	if err := os.MkdirAll(filepath.Join(f.root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := f.prompt("/waffle:platform ./docs/...")
	if !strings.Contains(out, "names no waffle service") || f.h.load("s1").Mode != "spec" {
		t.Errorf("context %q, state %+v", out, f.h.load("s1"))
	}
}

func TestAGrantReachesTheGuardUntilTheModeChanges(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:redgreen ./services/kostej")
	out := f.prompt("/waffle:allow gomod")
	if !strings.Contains(out, "granted gomod") || !strings.Contains(f.last(), "agents:grant redgreen gomod") {
		t.Fatalf("allow: %q, calls %v", out, f.calls)
	}

	f.edit(filepath.Join(f.svc, "go.mod"))
	if !strings.HasSuffix(f.last(), "--grant gomod") {
		t.Errorf("the guard was asked %q", f.last())
	}

	f.prompt("/waffle:steps")
	f.edit(filepath.Join(f.svc, "go.mod"))
	if strings.Contains(f.last(), "--grant") {
		t.Errorf("a grant outlived its mode: %q", f.last())
	}
}

func TestAGrantTheWaffleRejectsIsNotKept(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "agents:grant" {
			return "Error: the gomod area is not granted in steps mode", errors.New("exit status 1")
		}
		return allowing(args)
	})
	f.prompt("/waffle:steps ./services/kostej")
	f.prompt("/waffle:allow gomod")
	if granted := f.h.load("s1").Granted; len(granted) != 0 {
		t.Errorf("granted %v", granted)
	}
}

// A path in no service goes to every service the task names, and one that claims
// it decides.
func TestASharedPathIsAskedOfTheTasksServices(t *testing.T) {
	event := filepath.Join("shared", "api", "events", "price.go")
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "agents:guard" && strings.HasSuffix(args[2], event) {
			return "Error: redgreen mode does not allow it; spec mode does", errors.New("exit status 1")
		}
		return allowing(args)
	})
	f.prompt("/waffle:redgreen ./services/kostej")
	d := f.edit(filepath.Join(f.root, event))
	if d == nil || d.HookSpecificOutput == nil {
		t.Fatalf("decision = %+v, calls %v", d, f.calls)
	}
}

func TestASharedPathNoServiceClaimsIsNotChecked(t *testing.T) {
	f := newFixture(t, func(args []string) (string, error) {
		if args[0] == "agents:guard" {
			return "unclaimed: no area of this service names it", nil
		}
		return allowing(args)
	})
	f.prompt("/waffle:spec ./services/kostej ./services/shark")
	if d := f.edit(filepath.Join(f.root, "docs", "readme.md")); d != nil {
		t.Errorf("decision = %+v", d)
	}
	if len(f.calls) != 4 {
		t.Errorf("expected both services asked, calls %v", f.calls)
	}
}

// A service on a waffle released before the modes must stay editable.
func TestAnOldWaffleIsWarnedAboutOnceAndNeverBlocks(t *testing.T) {
	f := newFixture(t, func([]string) (string, error) {
		return `Unknown command "agents:guard"`, errors.New("exit status 1")
	})
	first := f.edit(filepath.Join(f.svc, "waffle.yaml"))
	second := f.edit(filepath.Join(f.svc, "waffle.yaml"))

	if first == nil || first.HookSpecificOutput != nil || !strings.Contains(first.SystemMessage, "no modes") {
		t.Errorf("first = %+v", first)
	}
	if second != nil {
		t.Errorf("second = %+v", second)
	}
}

func TestASessionFromBeforeOneModeIsCarriedOver(t *testing.T) {
	f := newFixture(t, allowing)
	old := `{"modes":{"` + f.svc + `":"steps"},"grants":{"` + f.svc + `":["model"]}}`
	path := f.h.file("s1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	st := f.h.load("s1")
	if st.modeOf(f.svc) != "steps" || len(st.Granted) != 1 || st.Granted[0] != "model" {
		t.Errorf("state %+v", st)
	}
}

func TestSessionStartAnnouncesThePluginAndTheMode(t *testing.T) {
	f := newFixture(t, allowing)
	f.prompt("/waffle:steps ./services/kostej")
	out := f.h.sessionStart(input{SessionID: "s1", Cwd: f.root})
	if !strings.Contains(out, "waffle plugin active") || !strings.Contains(out, "steps, for services/kostej") {
		t.Errorf("context:\n%s", out)
	}
}

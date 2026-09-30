package flows

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/gorib/waffle/tools/trace"
)

var update = flag.Bool("flows.update", false, "rewrite tests/flows.yaml from this run instead of checking it")

// A suite may move the working directory to isolate its files; the package
// directory is where the test binary started.
var home, _ = os.Getwd()

type Doc struct {
	Service  string   `yaml:"service"`
	Inputs   []Input  `yaml:"inputs"`
	Outputs  []Output `yaml:"outputs,omitempty"`
	Orphaned []Orphan `yaml:"orphaned,omitempty"`
}

type Input struct {
	Message     string   `yaml:"message"`
	Description string   `yaml:"description,omitempty"`
	Fields      []string `yaml:"fields,omitempty"`
	Branches    []Branch `yaml:"branches"`
}

type Branch struct {
	Sends       []string `yaml:"sends,omitempty"`
	Refusal     string   `yaml:"refused,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Cases       []string `yaml:"cases"`
}

func (b Branch) refused() bool { return b.Refusal != "" }

type Output struct {
	Message string   `yaml:"message"`
	Fields  []string `yaml:"fields,omitempty"`
}

type Orphan struct {
	Anchor      string `yaml:"anchor"`
	Description string `yaml:"description"`
}

func message(kind, name string) string { return kind + " " + name }

func (i Input) anchor() string { return i.Message }

func (b Branch) anchor(in Input) string {
	a := in.Message + " -> " + strings.Join(b.Sends, ", ")
	if b.refused() {
		a += " (refused " + b.Refusal + ")"
	}
	return a
}

// What each scenario received, and what it sent after it until the next thing
// it received.
func build(service string, records []trace.Record, cases features) Doc {
	type key struct {
		sends   string
		refusal string
	}
	inputs := map[string]*Input{}
	branches := map[string]map[key]*Branch{}
	outputs := map[string]map[string]bool{}
	fields := map[string]map[string]bool{}

	for _, run := range scenarios(records) {
		title := normal(run[0].Scenario)
		label := title
		if file := cases.file(run[0].Feature, title); file != "" {
			label = file + ": " + title
		}
		marked := slices.ContainsFunc(run, func(r trace.Record) bool { return r.Step != "" })
		acting := func(r trace.Record) bool { return !marked || r.Step == "Action" }
		last := -1
		sent := map[int][]string{} // in the order it happened
		for i, r := range run {
			switch {
			case r.Dir == "in" && acting(r):
				last = i
				sent[i] = []string{}
				m := message(r.Kind, r.Name)
				if inputs[m] == nil {
					inputs[m] = &Input{Message: m}
					branches[m] = map[key]*Branch{}
					fields[m] = map[string]bool{}
				}
				paths(r.Request, fields[m])
			case r.Dir == "out":
				m := message(r.Kind, r.Name)
				if outputs[m] == nil {
					outputs[m] = map[string]bool{}
				}
				paths(r.Request, outputs[m])
				if last >= 0 && acting(r) {
					sent[last] = append(sent[last], m)
				}
			}
		}
		for i, r := range run {
			if _, input := sent[i]; !input {
				continue
			}
			m := message(r.Kind, r.Name)
			k := key{sends: strings.Join(passes(sent[i]), "\n"), refusal: r.Error}
			b := branches[m][k]
			if b == nil {
				b = &Branch{Refusal: k.refusal}
				if k.sends != "" {
					b.Sends = strings.Split(k.sends, "\n")
				}
				branches[m][k] = b
			}
			if !slices.Contains(b.Cases, label) {
				b.Cases = append(b.Cases, label)
			}
		}
	}

	doc := Doc{Service: service}
	for _, m := range sorted(keys(inputs)) {
		in := *inputs[m]
		in.Fields = sorted(fields[m])
		for _, b := range branches[m] {
			slices.Sort(b.Cases)
			in.Branches = append(in.Branches, *b)
		}
		slices.SortFunc(in.Branches, func(a, b Branch) int { return strings.Compare(a.anchor(in), b.anchor(in)) })
		doc.Inputs = append(doc.Inputs, in)
	}
	for _, m := range sorted(keys(outputs)) {
		doc.Outputs = append(doc.Outputs, Output{Message: m, Fields: sorted(outputs[m])})
	}
	return doc
}

func passes(sends []string) []string {
	out := slices.Clone(sends)
	for folded := true; folded; {
		folded = false
		for size := 1; size <= len(out)/2 && !folded; size++ {
			for i := 0; i+2*size <= len(out); i++ {
				if slices.Equal(out[i:i+size], out[i+size:i+2*size]) {
					out = slices.Delete(out, i+size, i+2*size)
					folded = true
					break
				}
			}
		}
	}
	return out
}

// Records by scenario, each in the order its calls began. A record outside any
// scenario — a boot test, a migration check — belongs to no case.
func scenarios(records []trace.Record) [][]trace.Record {
	var order []string
	by := map[string][]trace.Record{}
	for _, r := range records {
		if r.Scenario == "" {
			continue
		}
		if by[r.Scenario] == nil {
			order = append(order, r.Scenario)
		}
		by[r.Scenario] = append(by[r.Scenario], r)
	}
	out := make([][]trace.Record, 0, len(order))
	for _, s := range order {
		run := by[s]
		slices.SortStableFunc(run, func(a, b trace.Record) int { return int(a.Seq - b.Seq) })
		out = append(out, run)
	}
	return out
}

// Carries the prose of old onto fresh by anchor; what finds no anchor is orphaned.
func reconcile(old, fresh Doc) Doc {
	prose := map[string]string{}
	for _, in := range old.Inputs {
		if in.Description != "" {
			prose[in.anchor()] = in.Description
		}
		for _, b := range in.Branches {
			if b.Description != "" {
				prose[b.anchor(in)] = b.Description
			}
		}
	}
	used := map[string]bool{}
	for i := range fresh.Inputs {
		in := &fresh.Inputs[i]
		if d, ok := prose[in.anchor()]; ok {
			in.Description, used[in.anchor()] = d, true
		}
		for j := range in.Branches {
			b := &in.Branches[j]
			if d, ok := prose[b.anchor(*in)]; ok {
				b.Description, used[b.anchor(*in)] = d, true
			}
		}
	}
	fresh.Orphaned = slices.Clone(old.Orphaned)
	for _, a := range sorted(keys(prose)) {
		if !used[a] {
			fresh.Orphaned = append(fresh.Orphaned, Orphan{Anchor: a, Description: prose[a]})
		}
	}
	return fresh
}

const header = "# Generated from the component suite by waffle. The structure is rewritten by\n" +
	"# `go test ./tests/... -args -flows.update`; only the description fields are yours.\n"

func encode(v any) []byte {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	_ = enc.Encode(v)
	return b.Bytes()
}

// Complete says whether the suite ran whole: a narrowed run saw part of the
// service, and its document would lose the rest.
func Complete() bool {
	for _, name := range []string{"test.run", "test.skip", "godog.tags"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return false
		}
	}
	// godog.paths is a Func flag, whose value prints empty whatever was given.
	return !slices.ContainsFunc(os.Args, func(arg string) bool {
		return strings.HasPrefix(strings.TrimLeft(arg, "-"), "godog.paths")
	})
}

// Check compares what the suite traced with the document at path, or rewrites
// the document under -flows.update. The cases are read from ./feature.
func Check(path, service string, records []trace.Record) error {
	path = filepath.Join(home, path)
	fresh := build(service, records, readFeatures(filepath.Join(home, "feature")))
	var old Doc
	current, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(current, &old); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(fresh.Inputs) == 0 && len(old.Inputs) == 0 {
		return nil
	}
	merged := reconcile(old, fresh)
	body := append([]byte(header), encode(merged)...)

	if *update {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}
		if len(merged.Orphaned) > 0 {
			return fmt.Errorf("%s: rewritten, and %d descriptions lost their anchor — see orphaned", path, len(merged.Orphaned))
		}
		return nil
	}
	if !bytes.Equal(current, body) {
		return fmt.Errorf("%s does not match what the suite traced:\n%s\nrewrite it with `go test ./tests/... -args -flows.update` and review the diff",
			path, summary(old, merged))
	}
	if len(merged.Orphaned) > 0 {
		return fmt.Errorf("%s: %d descriptions lost their anchor — delete them or move them under the input or branch they belong to now", path, len(merged.Orphaned))
	}
	return nil
}

// What changed, by anchor: enough to see a renamed key or a case gone without
// reading the whole file.
func summary(old, fresh Doc) string {
	anchors := func(d Doc) map[string]bool {
		out := map[string]bool{}
		for _, in := range d.Inputs {
			out["input "+in.anchor()] = true
			for _, b := range in.Branches {
				out["branch "+b.anchor(in)] = true
				for _, c := range b.Cases {
					out["case "+c+"  ["+b.anchor(in)+"]"] = true
				}
			}
		}
		for _, o := range d.Outputs {
			out["output "+o.Message] = true
		}
		return out
	}
	was, now := anchors(old), anchors(fresh)
	var lines []string
	for _, a := range sorted(was) {
		if !now[a] {
			lines = append(lines, "  - "+a)
		}
	}
	for _, a := range sorted(now) {
		if !was[a] {
			lines = append(lines, "  + "+a)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "  (the fields of a message changed)")
	}
	if len(lines) > 40 {
		lines = append(lines[:40], fmt.Sprintf("  … and %d more", len(lines)-40))
	}
	return strings.Join(lines, "\n")
}

func keys[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func sorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

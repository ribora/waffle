package flows

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/gorib/waffle/surface"
)

type Index struct {
	Flows    []Flow          `yaml:"flows"`
	Links    []Link          `yaml:"links,omitempty"`
	Untested []string        `yaml:"untested,omitempty"`
	Outside  []string        `yaml:"outside,omitempty"`
	System   *surface.Report `yaml:"system,omitempty"`
	Orphaned []Orphan        `yaml:"orphaned,omitempty"`
}

type Flow struct {
	Id          string   `yaml:"id"`
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Entry       string   `yaml:"entry"`
	From        []string `yaml:"from,omitempty"`
	Steps       []string `yaml:"steps"`
}

type Link struct {
	From    string   `yaml:"from"`
	To      string   `yaml:"to"`
	Message string   `yaml:"message"`
	Notes   []string `yaml:"notes,omitempty"`
}

type service struct {
	doc      Doc
	features features
}

type step struct {
	service string
	input   Input
}

func (s step) id() string { return s.service + ": " + s.input.Message }

type project struct {
	services map[string]service
	steps    map[string]step
	consumes map[string][]string // message -> step ids
	produces map[string][]string // message -> services
	surfaces []surface.Surface   // every service's, traced or not
}

func load(root string) (project, error) {
	p := project{services: map[string]service{}, steps: map[string]step{}, consumes: map[string][]string{}, produces: map[string][]string{}}
	err := filepath.WalkDir(root, func(dir string, d fs.DirEntry, err error) error {
		switch {
		case err != nil || !d.IsDir():
			return nil
		case dir != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "node_modules"):
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(dir, "waffle.yaml")); err != nil {
			return nil
		}
		if s, err := surface.Read(filepath.Join(dir, surface.File)); err == nil {
			p.surfaces = append(p.surfaces, s)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		body, err := os.ReadFile(filepath.Join(dir, "tests", "flows.yaml"))
		if errors.Is(err, os.ErrNotExist) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		var doc Doc
		if err := yaml.Unmarshal(body, &doc); err != nil {
			return fmt.Errorf("%s: %w", filepath.Join(dir, "tests", "flows.yaml"), err)
		}
		p.services[doc.Service] = service{doc: doc, features: readFeatures(filepath.Join(dir, "tests", "feature"))}
		return filepath.SkipDir
	})
	if err != nil {
		return p, err
	}
	for _, name := range sorted(keys(p.services)) {
		doc := p.services[name].doc
		for _, in := range doc.Inputs {
			s := step{service: name, input: in}
			p.steps[s.id()] = s
			p.consumes[in.Message] = append(p.consumes[in.Message], s.id())
		}
		for _, out := range doc.Outputs {
			p.produces[out.Message] = append(p.produces[out.Message], name)
		}
	}
	return p, nil
}

func kind(m string) string { k, _, _ := strings.Cut(m, " "); return k }

// Where a message goes that no traced case followed, by the code of the whole set.
func (p project) beyond(from, m string) string {
	if len(p.surfaces) == 0 {
		return "not traced"
	}
	k, name, _ := strings.Cut(m, " ")
	exact, wild := surface.Receivers(p.surfaces, k, name)
	if k != "event" {
		exact = slices.DeleteFunc(exact, func(s string) bool { return s == from })
	}
	switch {
	case len(exact) > 0:
		return "received by " + strings.Join(exact, ", ") + ", no case took it"
	case len(wild) > 0:
		return "only a wildcard subscription: " + strings.Join(wild, ", ")
	}
	return "out of the system"
}

func (p project) senders(s step) []string {
	k, name, _ := strings.Cut(s.input.Message, " ")
	return slices.DeleteFunc(surface.Senders(p.surfaces, k, name), func(from string) bool {
		return from == s.service && k != "event"
	})
}

func (p project) untested(traced []Link) []string {
	had := map[string]bool{}
	for _, l := range traced {
		had[l.From+" → "+l.To+": "+l.Message] = true
	}
	var out []string
	add := func(from, to, m string) {
		if _, ok := p.services[to]; ok && !had[from+" → "+to+": "+m] {
			out = append(out, from+" → "+to+": "+m)
		}
	}
	for _, s := range p.surfaces {
		if _, ok := p.services[s.Service]; !ok {
			continue
		}
		for _, key := range s.Publishes {
			exact, _ := surface.Receivers(p.surfaces, "event", key)
			for _, to := range exact {
				add(s.Service, to, "event "+key)
			}
		}
		for _, m := range s.Calls {
			if !strings.HasPrefix(m, "/") {
				continue // outgoing HTTP is not traced, so no case could follow it
			}
			exact, _ := surface.Receivers(p.surfaces, "grpc", m)
			for _, to := range exact {
				if to != s.Service {
					add(s.Service, to, "grpc "+m)
				}
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// A call goes to another service; an event may come back to the one that sent
// it, since a service consumes its own events through the bus like anyone's.
func (p project) receivers(from, m string) []string {
	var out []string
	for _, id := range p.consumes[m] {
		if p.steps[id].service != from || kind(m) == "event" {
			out = append(out, id)
		}
	}
	return out
}

func (p project) entry(s step) bool {
	switch kind(s.input.Message) {
	case "http", "command":
		return true
	}
	for _, from := range p.produces[s.input.Message] {
		if from != s.service || kind(s.input.Message) == "event" {
			return false
		}
	}
	return true
}

// Everything reachable from the entry through the branches that were not
// refused, depth first: a step comes right after the first message that
// reaches it, the way the diagram draws it. onward says whether anything did.
func (p project) walk(entry string) (order []string, onward bool) {
	seen := map[string]bool{}
	var visit func(id string)
	visit = func(id string) {
		seen[id] = true
		order = append(order, id)
		s := p.steps[id]
		for _, sent := range sequence(s.input.Branches) {
			for _, to := range p.receivers(s.service, sent.message) {
				onward = true
				if !seen[to] {
					visit(to)
				}
			}
		}
	}
	visit(entry)
	return order, onward
}

// A message as a branch sends it: the nth time that branch sends it.
type send struct {
	message string
	nth     int
}

func sends(messages []string) []send {
	seen := map[string]int{}
	out := make([]send, 0, len(messages))
	for _, m := range messages {
		seen[m]++
		out = append(out, send{message: m, nth: seen[m]})
	}
	return out
}

// A before B when some case sent A first; ties and conflicts go alphabetically.
func sequence(branches []Branch) []send {
	after := map[send]map[send]bool{}
	before := map[send]int{}
	nodes := map[send]bool{}
	for _, b := range branches {
		if b.refused() {
			continue
		}
		order := sends(b.Sends)
		for i, s := range order {
			nodes[s] = true
			for _, later := range order[i+1:] {
				if after[s] == nil {
					after[s] = map[send]bool{}
				}
				if !after[s][later] {
					after[s][later] = true
					before[later]++
				}
			}
		}
	}
	left := make([]send, 0, len(nodes))
	for s := range nodes {
		left = append(left, s)
	}
	slices.SortFunc(left, func(a, b send) int {
		if c := strings.Compare(a.message, b.message); c != 0 {
			return c
		}
		return a.nth - b.nth
	})
	var out []send
	for len(left) > 0 {
		pick := slices.IndexFunc(left, func(s send) bool { return before[s] == 0 })
		if pick < 0 {
			pick = 0
		}
		s := left[pick]
		left = slices.Delete(left, pick, pick+1)
		out = append(out, s)
		for later := range after[s] {
			before[later]--
		}
	}
	return out
}

var unsafe = regexp.MustCompile(`[^a-z0-9]+`)

// A page is named by its step, which stays put; a title is prose and may change.
func slug(s step) string {
	return strings.Trim(unsafe.ReplaceAllString(strings.ToLower(s.service+"-"+kind(s.input.Message)+"-"+short(s.input.Message)), "-"), "-")
}

// The name of a message without its package: Service.Method for a call.
func short(m string) string {
	k, name, _ := strings.Cut(m, " ")
	if k != "grpc" {
		return name
	}
	parts := strings.Split(strings.TrimPrefix(name, "/"), "/")
	if len(parts) != 2 {
		return name
	}
	return parts[0][strings.LastIndex(parts[0], ".")+1:] + "." + parts[1]
}

const pageHeader = "<!-- Generated by `waffle flows` from the services' tests/flows.yaml. Do not edit:\n" +
	"     prose goes into the description fields of those files and of the index. -->\n"

// Generated says whether a page is one Assemble writes.
func Generated(page []byte) bool { return bytes.HasPrefix(page, []byte(pageHeader)) }

// Assemble builds the index at path and a page per input a flow reaches beside
// it, keeping the prose the index at path already has.
func Assemble(root, path string) (map[string][]byte, error) {
	p, err := load(root)
	if err != nil {
		return nil, err
	}
	if len(p.services) == 0 && len(p.surfaces) == 0 {
		return nil, fmt.Errorf("no service below %s has tests/flows.yaml or %s: run its suite with -flows.update, or `waffle flows record`", root, surface.File)
	}
	var old Index
	if body, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(body, &old); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	prose := map[string]Flow{}
	for _, f := range old.Flows {
		prose[f.Id] = f
	}

	files := map[string][]byte{}
	index := Index{Orphaned: slices.Clone(old.Orphaned)}
	used := map[string]bool{}
	reached := map[string]bool{}
	for _, id := range sorted(keys(p.steps)) {
		s := p.steps[id]
		if !p.entry(s) {
			continue
		}
		order, onward := p.walk(id)
		if !onward {
			continue
		}
		f := Flow{Id: slug(s), Entry: id, From: p.senders(s), Steps: order}
		if was, ok := prose[f.Id]; ok {
			f.Title, f.Description, used[f.Id] = was.Title, was.Description, true
		}
		index.Flows = append(index.Flows, f)
		files[filepath.Join(filepath.Dir(path), f.Id+".md")] = page(p, f, true)
		for _, step := range order[1:] {
			reached[step] = true
		}
	}
	for _, id := range sorted(reached) {
		order, _ := p.walk(id)
		f := Flow{Id: slug(p.steps[id]), Entry: id, Steps: order}
		files[filepath.Join(filepath.Dir(path), f.Id+".md")] = page(p, f, false)
	}
	for _, id := range sorted(keys(prose)) {
		if !used[id] && (prose[id].Title != "" || prose[id].Description != "") {
			index.Orphaned = append(index.Orphaned, Orphan{Anchor: "flow " + id, Description: strings.TrimSpace(prose[id].Title + "\n" + prose[id].Description)})
		}
	}
	index.Links, index.Outside = p.links()
	index.Untested = p.untested(index.Links)
	if len(p.surfaces) > 0 {
		report, err := surface.Verify(p.surfaces)
		if err != nil {
			return nil, err
		}
		index.System = &report
	}
	files[path] = append([]byte("# Generated by `waffle flows`. Only the title and description of a flow are yours.\n"), encode(index)...)
	return files, nil
}

func (p project) links() ([]Link, []string) {
	var links []Link
	var outside []string
	for _, from := range sorted(keys(p.services)) {
		for _, out := range p.services[from].doc.Outputs {
			receivers := p.receivers(from, out.Message)
			if len(receivers) == 0 {
				outside = append(outside, from+" sends "+out.Message+": "+p.beyond(from, out.Message))
			}
			sent := set(out.Fields)
			for _, to := range receivers {
				l := Link{From: from, To: p.steps[to].service, Message: out.Message}
				if extra := missing(set(p.steps[to].input.Fields), sent); len(extra) > 0 {
					l.Notes = append(l.Notes, "the receiver's cases send fields the sender never does: "+strings.Join(extra, ", "))
				}
				links = append(links, l)
			}
		}
	}
	return links, outside
}

// LinkChanges says, between two versions of an index, which links between
// services were lost and which appeared: the reason a page changed.
func LinkChanges(was, now []byte) []string {
	var before, after Index
	if yaml.Unmarshal(was, &before) != nil || yaml.Unmarshal(now, &after) != nil {
		return nil
	}
	name := func(l Link) string { return fmt.Sprintf("%s → %s: %s", l.From, l.To, l.Message) }
	had, has := map[string]bool{}, map[string]bool{}
	for _, l := range before.Links {
		had[name(l)] = true
	}
	for _, l := range after.Links {
		has[name(l)] = true
	}
	var out []string
	for _, l := range sorted(had) {
		if !has[l] {
			out = append(out, "link lost: "+l)
		}
	}
	for _, l := range sorted(has) {
		if !had[l] {
			out = append(out, "link new: "+l)
		}
	}
	return out
}

// Paths one side relies on and the other never shows. A scalar where the other
// side has an object is a case feeding a broken message, not a field.
func missing(has, lacks map[string]bool) []string {
	var out []string
	for _, pth := range sorted(has) {
		if lacks[pth] {
			continue
		}
		broken := false
		for q := range lacks {
			if strings.HasPrefix(q, pth+".") || strings.HasPrefix(q, pth+"[]") {
				broken = true
				break
			}
		}
		if !broken {
			out = append(out, pth)
		}
	}
	return out
}

func set(list []string) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, s := range list {
		out[s] = true
	}
	return out
}

package main

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// The manifest rules from docs/PLUGINS.md (docs/schema/jaira-plugin.schema.json
// in the jaira repository). The catalogue's CI checks the same; failing here
// is cheaper than failing there.
func TestManifestFollowsTheStandard(t *testing.T) {
	var m struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		Version         string `json:"version"`
		MinJairaVersion string `json:"minJairaVersion"`
		Commands        []struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
		} `json:"commands"`
		Capabilities []string `json:"capabilities"`
		Events       []string `json:"events"`
	}
	dec := json.NewDecoder(bytes.NewReader(manifestJSON))
	dec.DisallowUnknownFields() // "web" is allowed by the standard; add it here when you use it
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("jaira-plugin.json: %v", err)
	}

	idRe := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	if !idRe.MatchString(m.ID) || len(m.ID) > 40 {
		t.Errorf("id %q: lower-case letters, digits and single dashes, starting with a letter", m.ID)
	}
	if strings.Contains(m.ID, "jaira") {
		t.Errorf("id %q: must not contain 'jaira' - the program is already called jaira-<id>", m.ID)
	}
	if m.Name == "" || len(m.Name) > 80 {
		t.Errorf("name %q: 1 to 80 characters", m.Name)
	}
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`).MatchString(m.Version) {
		t.Errorf("version %q: semver without a leading v", m.Version)
	}
	if _, ok := parseVersion(m.MinJairaVersion); !ok || strings.ContainsAny(m.MinJairaVersion, "v-+") {
		t.Errorf("minJairaVersion %q: X.Y.Z", m.MinJairaVersion)
	}
	if m.Commands == nil {
		t.Error("commands: required, may be empty")
	}
	for _, c := range m.Commands {
		if !idRe.MatchString(c.Name) || c.Summary == "" || len(c.Summary) > 120 {
			t.Errorf("command %q: name like an id, summary 1 to 120 characters", c.Name)
		}
	}
	allowed := map[string]bool{"read": true, "write": true, "network": true, "secrets": true, "web": true}
	if m.Capabilities == nil {
		t.Error("capabilities: required, may be empty")
	}
	for _, c := range m.Capabilities {
		if !allowed[c] {
			t.Errorf("capability %q: one of read, write, network, secrets, web", c)
		}
	}
	events := map[string]bool{"create": true, "move": true, "claim": true, "set": true}
	for _, e := range m.Events {
		if !events[e] {
			t.Errorf("event %q: one of create, move, claim, set", e)
		}
	}
}

func TestManifestCommandPrintsTheFile(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"manifest", "--json"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if !bytes.Equal(out.Bytes(), manifestJSON) {
		t.Errorf("manifest --json printed something other than jaira-plugin.json")
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"nope"}, &out, &errOut); code != exitUsage {
		t.Errorf("exit %d, want %d", code, exitUsage)
	}
}

func TestTooOldJairaIsRefused(t *testing.T) {
	t.Setenv("JAIRA_VERSION", "0.0.1")
	var out, errOut bytes.Buffer
	if code := run([]string{"greet"}, &out, &errOut); code != exitRefused {
		t.Errorf("exit %d, want %d", code, exitRefused)
	}
	if !strings.Contains(errOut.String(), "self upgrade") {
		t.Errorf("stderr %q does not say what to do", errOut.String())
	}
}

func TestCheckJairaVersion(t *testing.T) {
	for _, c := range []struct {
		running, minimum string
		refused          bool
	}{
		{"", "0.5.10", false},     // started by hand
		{"dev", "0.5.10", false},  // development build
		{"0.5.9", "0.5.10", true}, // numeric, not string, comparison
		{"0.5.10", "0.5.10", false},
		{"0.6.0-rc1", "0.5.10", false},
		{"1.0.0", "0.5.10", false},
	} {
		if got := checkJairaVersion(c.running, c.minimum) != ""; got != c.refused {
			t.Errorf("checkJairaVersion(%q, %q) refused=%v, want %v", c.running, c.minimum, got, c.refused)
		}
	}
}

const board = `{"count": 2, "tickets": [
  {"id": "01ABCDEFGHJKMNPQRSTVWX1234", "handle": "X1234A", "title": "first"},
  {"id": "01ABCDEFGHJKMNPQRSTVWX5678", "handle": "Y5678B", "title": "second"}
]}`

// fakeJaira records the calls and answers 'list' with board.
func fakeJaira(t *testing.T) *[][]string {
	t.Helper()
	calls := &[][]string{}
	old := runJaira
	runJaira = func(args ...string) ([]byte, error) {
		*calls = append(*calls, args)
		if args[0] == "list" {
			return []byte(board), nil
		}
		return []byte(`{}`), nil
	}
	t.Cleanup(func() { runJaira = old })
	t.Setenv("JAIRA_VERSION", "")
	return calls
}

func TestGreetCountsTheBoard(t *testing.T) {
	calls := fakeJaira(t)
	t.Setenv("JAIRA_BOARD", "video")
	var out, errOut bytes.Buffer
	if code := run([]string{"greet"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "2 tickets") {
		t.Errorf("stdout %q", out.String())
	}
	if got := strings.Join((*calls)[0], " "); got != "list --json --board video" {
		t.Errorf("called 'jaira %s', want the board from JAIRA_BOARD", got)
	}
	if len(*calls) != 1 {
		t.Errorf("greet without a ticket wrote something: %v", *calls)
	}
}

func TestGreetWritesANoteThroughTheCLI(t *testing.T) {
	calls := fakeJaira(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"greet", "y5678b"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if len(*calls) != 2 {
		t.Fatalf("calls %v", *calls)
	}
	note := (*calls)[1]
	if note[0] != "note" || note[1] != "Y5678B" || note[len(note)-1] != "--json" {
		t.Errorf("called 'jaira %s'", strings.Join(note, " "))
	}
}

func TestGreetUnknownTicketIsNotFound(t *testing.T) {
	fakeJaira(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"greet", "ZZZZZZ"}, &out, &errOut); code != exitNotFound {
		t.Errorf("exit %d, want %d", code, exitNotFound)
	}
}

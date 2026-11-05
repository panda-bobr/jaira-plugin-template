// jaira-hello is the example plugin of the jaira plugin template.
//
// It shows the three things every plugin does: print its manifest
// ("manifest --json"), read the board through jaira's read commands, and
// write through jaira's write commands — never into .jaira/ directly.
// The standard it follows is docs/PLUGINS.md in the jaira repository.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// manifestJSON is jaira-plugin.json from the repository root, built into the
// binary so that the file and "manifest --json" can never disagree.
//
//go:embed jaira-plugin.json
var manifestJSON []byte

// Exit codes, the same contract as jaira's own.
const (
	exitOK       = 0
	exitError    = 1
	exitUsage    = 2
	exitRefused  = 3
	exitNotFound = 5
)

type manifest struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	MinJairaVersion string `json:"minJairaVersion"`
}

// runJaira calls the jaira binary and returns its stdout. Tests replace it.
var runJaira = func(args ...string) ([]byte, error) {
	cmd := exec.Command("jaira", args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var m manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		fmt.Fprintf(stderr, "jaira-plugin.json: %v\n", err)
		return exitError
	}
	if len(args) == 0 {
		usage(stderr, m)
		return exitUsage
	}
	switch args[0] {
	case "manifest":
		// Always JSON; --json is accepted because that is how jaira asks.
		stdout.Write(manifestJSON)
		return exitOK
	case "version", "--version":
		fmt.Fprintln(stdout, m.Version)
		return exitOK
	case "-h", "--help", "help":
		usage(stdout, m)
		return exitOK
	}

	if msg := checkJairaVersion(os.Getenv("JAIRA_VERSION"), m.MinJairaVersion); msg != "" {
		fmt.Fprintf(stderr, "jaira-%s: %s\n", m.ID, msg)
		return exitRefused
	}

	switch args[0] {
	case "greet":
		return greet(m, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "jaira-%s: unknown command %q\n", m.ID, args[0])
		usage(stderr, m)
		return exitUsage
	}
}

func usage(w io.Writer, m manifest) {
	fmt.Fprintf(w, `%s %s - example jaira plugin

Usage:
  jaira %s greet             count the tickets on the board
  jaira %s greet <ticket>    leave a hello note on one ticket
  jaira-%s manifest --json   print jaira-plugin.json
`, m.Name, m.Version, m.ID, m.ID, m.ID)
}

// greet reads the board with "jaira list --json" and, given a ticket, writes
// a note on it with "jaira note".
func greet(m manifest, args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintf(stderr, "jaira-%s: greet takes at most one ticket\n", m.ID)
		return exitUsage
	}

	listArgs := []string{"list", "--json"}
	if board := os.Getenv("JAIRA_BOARD"); board != "" {
		listArgs = append(listArgs, "--board", board)
	}
	out, err := runJaira(listArgs...)
	if err != nil {
		return jairaFailed(m, "list", err, stderr)
	}
	var list struct {
		Count   int `json:"count"`
		Tickets []struct {
			ID     string `json:"id"`
			Handle string `json:"handle"`
			Title  string `json:"title"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		fmt.Fprintf(stderr, "jaira-%s: cannot read 'jaira list --json': %v\n", m.ID, err)
		return exitError
	}

	if len(args) == 0 {
		fmt.Fprintf(stdout, "Hello! This board has %d tickets.\n", list.Count)
		return exitOK
	}

	want := strings.ToUpper(args[0])
	for _, t := range list.Tickets {
		if strings.ToUpper(t.Handle) != want && strings.ToUpper(t.ID) != want {
			continue
		}
		note := fmt.Sprintf("Hello from the %s plugin %s.", m.Name, m.Version)
		if _, err := runJaira("note", t.Handle, note, "--json"); err != nil {
			return jairaFailed(m, "note", err, stderr)
		}
		fmt.Fprintf(stdout, "Left a note on %s: %s\n", t.Handle, t.Title)
		return exitOK
	}
	fmt.Fprintf(stderr, "jaira-%s: no ticket %s on this board; 'jaira list' shows them\n", m.ID, args[0])
	return exitNotFound
}

// jairaFailed passes jaira's own exit code on, so an agent can branch on it.
func jairaFailed(m manifest, cmd string, err error, stderr io.Writer) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	fmt.Fprintf(stderr, "jaira-%s: cannot run 'jaira %s': %v\n", m.ID, cmd, err)
	return exitError
}

// checkJairaVersion returns why the running jaira is too old, or "".
// Started by hand, without jaira, JAIRA_VERSION is empty and nothing is
// checked; a version that is not X.Y.Z (a development build) is let through.
func checkJairaVersion(running, minimum string) string {
	have, ok := parseVersion(running)
	if !ok {
		return ""
	}
	need, ok := parseVersion(minimum)
	if !ok {
		return ""
	}
	for i := range have {
		if have[i] != need[i] {
			if have[i] < need[i] {
				return fmt.Sprintf("needs jaira %s or newer, this is %s; run 'jaira self upgrade'", minimum, running)
			}
			return ""
		}
	}
	return ""
}

func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// Command termux-fonts changes the Termux terminal font, either through
// an interactive Bubble Tea picker or non-interactive --list / --apply
// flags for scripting.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"termux-fonts-go/internal/apply"
	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/scan"
	"termux-fonts-go/internal/tui"
)

// ensureBuiltinSeed copies the active regular slot file to
// fonts/Current.ttf when the library is empty, so first launch is never
// empty. It returns the seed path, or "" when no seeding was needed.
func ensureBuiltinSeed() string {
	entries, err := scan.ListLibrary()
	if err == nil && len(entries) > 0 {
		return ""
	}
	src, err := paths.FontSlotPath("regular")
	if err != nil {
		return ""
	}
	fi, err := os.Stat(src)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	dest := filepath.Join(paths.FontsDir(), "Current.ttf")
	if fi, err := os.Stat(dest); err == nil && fi.Mode().IsRegular() {
		return dest
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return ""
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return ""
	}
	if err := os.WriteFile(dest, data, fi.Mode().Perm()); err != nil {
		return ""
	}
	return dest
}

// casefoldHits returns every entry whose Name equals name
// case-insensitively, in library order.
func casefoldHits(entries []scan.FontEntry, name string) []scan.FontEntry {
	var hits []scan.FontEntry
	for _, e := range entries {
		if strings.EqualFold(e.Name, name) {
			hits = append(hits, e)
		}
	}
	return hits
}

// resolveMatch finds the library entry named name: an exact hit wins,
// otherwise a case-insensitive hit wins. It returns the entry plus
// (ambiguous, found): ambiguous when several case-insensitive names
// collide, found=false when nothing matches.
func resolveMatch(entries []scan.FontEntry, name string) (scan.FontEntry, bool, bool) {
	for _, e := range entries {
		if e.Name == name {
			return e, false, true
		}
	}
	switch hits := casefoldHits(entries, name); len(hits) {
	case 0:
		return scan.FontEntry{}, false, false
	case 1:
		return hits[0], false, true
	default:
		return scan.FontEntry{}, true, true
	}
}

func runList(entries []scan.FontEntry) {
	for _, e := range entries {
		fmt.Println(e.Name)
	}
}

func runApply(entries []scan.FontEntry, name, slot string) int {
	match, ambiguous, found := resolveMatch(entries, name)
	if !found {
		fmt.Fprintf(os.Stderr, "unknown font: %q\n", name)
		return 2
	}
	if ambiguous {
		names := []string{}
		for _, e := range casefoldHits(entries, name) {
			names = append(names, e.Name)
		}
		sort.Strings(names)
		fmt.Fprintf(os.Stderr, "ambiguous font %q; matches: %s\n", name, strings.Join(names, ", "))
		return 2
	}
	target, err := apply.InstallFont(match.Path, slot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if apply.LastReloadOK() != nil && !*apply.LastReloadOK() {
		fmt.Fprintf(os.Stderr, "warning: %s\n", apply.ManualRestartHint)
	}
	fmt.Printf("Applied %s to %s (%s)\n", match.Name, slot, target)
	return 0
}

func main() {
	os.Exit(realMain(os.Args[1:]))
}

func realMain(argv []string) int {
	ensureBuiltinSeed()
	fs := flag.NewFlagSet("termux-fonts", flag.ContinueOnError)
	list := fs.Bool("list", false, "Print library font names and exit.")
	applyName := fs.String("apply", "", "Install library font NAME into --slot and exit.")
	slot := fs.String("slot", "regular", "Font slot for --apply (default: regular).")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if _, err := paths.FontSlotPath(*slot); err != nil {
		fmt.Fprintf(os.Stderr, "%v (choose from %s)\n", err, strings.Join(paths.SlotNames, ", "))
		return 2
	}
	entries, err := scan.ListLibrary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot list library: %v\n", err)
		return 1
	}
	if *list {
		runList(entries)
		return 0
	}
	applyGiven := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "apply" {
			applyGiven = true
		}
	})
	if applyGiven {
		return runApply(entries, *applyName, *slot)
	}
	if _, err := tea.NewProgram(tui.InitialModel()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

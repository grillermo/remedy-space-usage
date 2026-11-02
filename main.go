package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/grillermo/chicle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "remedy-space-usage:", err)
		os.Exit(1)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", false, "scan and choose as usual, then show what would happen without deleting anything")
	useDefaults := flag.Bool("defaults", false, "start from the built-in ticks instead of your remembered selection")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	path := memoryPath(home)
	memory, err := loadMemory(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "remedy-space-usage: ignoring remembered selection:", err)
	}
	starting := memory
	if *useDefaults {
		starting = Memory{}
	}

	skips := parseSkips(skipFile, home)
	items := withoutSkipped(catalog(home, systemOutput), skips)
	starting.apply(items)
	scanner := newScanner(items, func() []*Item {
		var claimed []string
		for _, it := range items {
			claimed = append(claimed, it.SizePaths...)
		}
		found := discover(filepath.Join(home, "c"))
		found = append(found, largeEntries(filepath.Join(home, "Library/Caches"), claimed)...)
		found = append(found, largeEntries(filepath.Join(home, ".cache"), claimed)...)
		found = withoutSkipped(found, skips)
		starting.apply(found)
		return found
	})

	chosen, ticked, err := pick(scanner)
	if err != nil {
		return err
	}
	scanner.remember(ticked, memory)
	if err := memory.save(path); err != nil {
		fmt.Fprintln(os.Stderr, "remedy-space-usage: couldn't remember selection:", err)
	}
	if len(chosen) == 0 {
		return nil
	}
	if *dryRun {
		dryRunReport(os.Stdout, chosen, systemGuards())
		return nil
	}
	if !review(chosen) {
		fmt.Println("Nothing deleted.")
		return nil
	}
	execute(chosen)
	return nil
}

// pick returns the items to clean, nil if you quit, and the row keys ticked when
// the list closed, whichever way it closed. chicle.Run reports no ticks on a
// quit, so this runs the chicle model directly.
func pick(scanner *Scanner) ([]*Item, map[string]bool, error) {
	initial := scanner.snapshot()
	scanner.markApplied(initial)
	program := tea.NewProgram(chicle.New(chicle.Config{
		Title:       "Free up space: enter ticks, tab reaches the buttons, ctrl+d cleans (once the scan ends)",
		MultiSelect: true,
		NoCycle:     true,
		Columns:     []chicle.Column{{Title: "Size", Width: 9}, {Title: "Item", Width: 38}, {Title: "Where"}},
		Rows:        initial,
		Updates:     scanner.start(),
		Actions: []chicle.Action{
			{
				Label: "Clean",
				Key:   "ctrl+d",
				Show:  func(chicle.Selection) bool { return scanner.finished() },
				Run: func(sel chicle.Selection) chicle.Outcome {
					if len(sel.Ticked) == 0 {
						return chicle.Outcome{Status: "Nothing ticked"}
					}
					return chicle.Outcome{Result: "clean", Done: true}
				},
			},
			{Label: "Quit"},
		},
	}), tea.WithAltScreen())

	final, err := program.Run()
	if err != nil {
		return nil, nil, err
	}
	model := final.(chicle.Model)
	ticked := map[string]bool{}
	for _, r := range model.Selection().Ticked {
		ticked[r.Key] = true
	}
	if model.Result() != "clean" {
		return nil, ticked, nil
	}
	return scanner.selected(ticked), ticked, nil
}

// printPlan lists what each item would do and returns their combined size.
func printPlan(w io.Writer, items []*Item) int64 {
	var total int64
	for _, it := range items {
		total += it.SizeKB
		fmt.Fprintf(w, "\n%s  (%s)\n", it.Label, it.sizeLabel())
		for _, line := range describe(it) {
			fmt.Fprintln(w, "  "+line)
		}
	}
	return total
}

func review(items []*Item) bool {
	total := printPlan(os.Stdout, items)
	fmt.Printf("\nThis permanently deletes up to %s. Continue? [y/N] ", formatSize(total))
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(answer), "y")
}

// dryRunReport shows the same plan as review, and which items would be skipped
// right now. It only reads: nothing here deletes or runs a cleanup.
func dryRunReport(w io.Writer, items []*Item, g guards) {
	var active []*Item
	for _, it := range items {
		if reason := g.blockReason(it); reason != "" {
			fmt.Fprintf(w, "SKIP  %s: %s\n", it.Label, reason)
			continue
		}
		active = append(active, it)
	}
	total := printPlan(w, active)
	fmt.Fprintf(w, "\nDry run: nothing was deleted. A real run would delete up to %s.\n", formatSize(total))
}

func execute(items []*Item) {
	start := freeKB()
	g := systemGuards()
	failed := 0
	var total int64
	for _, it := range items {
		if reason := g.blockReason(it); reason != "" {
			fmt.Printf("SKIP  %-38s %s\n", it.Label, reason)
			continue
		}
		fmt.Printf("...   %s\n", it.Label)
		before := freeKB()
		err := perform(it)
		saved := savedKB(it, freeKB()-before)
		total += saved
		if err != nil {
			failed++
			fmt.Printf("FAIL  %-38s saved %s: %v\n", it.Label, formatSize(saved), err)
			continue
		}
		fmt.Printf("OK    %-38s saved %s\n", it.Label, formatSize(saved))
	}
	fmt.Printf("\nSaved %s in total (free disk space changed by %s).", formatSize(total), formatSize(max(freeKB()-start, 0)))
	if failed > 0 {
		fmt.Printf(" %d failed, see above.", failed)
	}
	fmt.Println()
}

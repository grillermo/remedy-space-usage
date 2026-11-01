package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grillermo/chicle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "remedy-space-usage:", err)
		os.Exit(1)
	}
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	items := catalog(home, systemOutput)
	scanner := newScanner(items, func() []*Item {
		var claimed []string
		for _, it := range items {
			claimed = append(claimed, it.SizePaths...)
		}
		found := discover(filepath.Join(home, "c"))
		found = append(found, largeEntries(filepath.Join(home, "Library/Caches"), claimed)...)
		return append(found, largeEntries(filepath.Join(home, ".cache"), claimed)...)
	})

	chosen, err := pick(scanner)
	if err != nil || len(chosen) == 0 {
		return err
	}
	if !review(chosen) {
		fmt.Println("Nothing deleted.")
		return nil
	}
	execute(chosen)
	return nil
}

func pick(scanner *Scanner) ([]*Item, error) {
	initial := scanner.snapshot()
	keys := map[string]bool{}
	result, err := chicle.Run(chicle.Config{
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
					for _, r := range sel.Ticked {
						keys[r.Key] = true
					}
					return chicle.Outcome{Result: "clean", Done: true}
				},
			},
			{Label: "Quit"},
		},
	})
	if err != nil || result != "clean" {
		return nil, err
	}
	return scanner.selected(keys), nil
}

func review(items []*Item) bool {
	var total int64
	for _, it := range items {
		total += it.SizeKB
		fmt.Printf("\n%s  (%s)\n", it.Label, it.sizeLabel())
		for _, line := range describe(it) {
			fmt.Println("  " + line)
		}
	}
	fmt.Printf("\nThis permanently deletes up to %s. Continue? [y/N] ", formatSize(total))
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(answer), "y")
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

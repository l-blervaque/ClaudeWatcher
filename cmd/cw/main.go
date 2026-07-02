package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ludo/claudewatcher/internal/scanner"
	"github.com/ludo/claudewatcher/internal/tui"
	"github.com/ludo/claudewatcher/internal/version"
)

func main() {
	diagnose := flag.Bool("diagnose", false, "print live-session process attribution and exit (read-only)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("cw v" + version.Full())
		return
	}

	if *diagnose {
		diags, err := scanner.Diagnose()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(scanner.FormatDiagnosis(diags))
		return
	}

	p := tea.NewProgram(tui.NewModel(), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

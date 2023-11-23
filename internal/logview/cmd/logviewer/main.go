package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/amonks/run/internal/logview"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	rate = flag.Int("rate", 2, "outputs per second")
)

func main() {
	flag.Parse()

	lv := logview.New()
	program := tea.NewProgram(
		lv,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	go func() {
		n := 0
		dur := time.Second / time.Duration(*rate)
		for {
			log := fmt.Sprintf("%d: %d The quick brown fox jumped over the lazy dog.", n, 1)
			program.Send(lv.WriteMsg(log))
			time.Sleep(dur)

			log = fmt.Sprintf(" %d The quick brown fox jumped over the lazy dog.", 2)
			program.Send(lv.WriteMsg(log))
			time.Sleep(dur)

			log = fmt.Sprintf(" %d The quick brown fox jumped over the lazy dog.\n", 3)
			program.Send(lv.WriteMsg(log))
			time.Sleep(dur)

			n++
		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

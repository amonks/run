package main

import (
	"fmt"
	"os"
	"time"

	"github.com/amonks/run/internal/logview"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	lv := logview.New()
	program := tea.NewProgram(
		lv,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	n := 0
	go func() {
		dur := 500 * time.Millisecond
		for {
			n++
			log := fmt.Sprintf("%d:", n)
			program.Send(lv.WriteMsg(log))
			time.Sleep(dur)

			for i := 1; i <= 2; i++ {
				log := fmt.Sprintf(" %d", i)
				program.Send(lv.WriteMsg(log))
				time.Sleep(dur)
			}

			program.Send(lv.WriteMsg(" 3\n"))
			time.Sleep(dur)

		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}


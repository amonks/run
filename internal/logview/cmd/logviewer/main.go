package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/amonks/run/internal/logview"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	lv := logview.New()
	program := tea.NewProgram(
		lv,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for {
			if !sc.Scan() {
				if err := sc.Err(); err != nil {
					program.Send(lv.WriteMsg(sc.Err().Error()))
				} else {
					program.Send(lv.WriteMsg("EOF\n"))
				}
				break
			}
			program.Send(lv.WriteMsg(sc.Text() + "\n"))
		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	if err := os.Stdin.Close(); err != nil {
		fmt.Println("error closing stdin")
	}

	os.Exit(0)
}

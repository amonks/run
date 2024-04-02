package main

import (
	"bufio"
	"io"
	"os"
	"time"
)

type Sink func(string)

func (sink Sink) tailStdin() error {
	sc := bufio.NewScanner(os.Stdin)
	defer os.Stdin.Close()
	for {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return err
			} else {
				sink("EOF\n")
			}
			break
		}
		sink(sc.Text() + "\n")
	}
	return nil
}

func (sink Sink) tailFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		bs, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		sink(string(bs))
		time.Sleep(time.Millisecond * 32)
	}
}

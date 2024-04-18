package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

var logfile = flag.String("logfile", "mutex.log", "path to mutex.log file to consider")

func run() error {
	flag.Parse()

	file, err := os.Open(*logfile)
	if err != nil {
		return err
	}
	defer file.Close()

	l := &lockfinder{
		m: map[string]string{},
	}

	scn := bufio.NewScanner(file)
	for scn.Scan() {
		l.handleLine(scn.Text())
	}
	if err := scn.Err(); err != nil {
		return err
	}

	fmt.Println(l.report())
	return nil
}

type lockfinder struct {
	m map[string]string
}

var re = regexp.MustCompile(`(?P<Date>.{25}) \[(?P<Lock>.+)](?P<Fn>.*) (?P<Op>.*s) lock$`)

func (l *lockfinder) handleLine(line string) error {
	match := re.FindStringSubmatch(line)
	if len(match) == 0 {
		return nil
	}
	lock, fn, op := match[re.SubexpIndex("Lock")], match[re.SubexpIndex("Fn")], match[re.SubexpIndex("Op")]
	switch op {
	case "seeks":
	case "receives":
		l.m[lock] = fn
	case "releases":
		l.m[lock] = ""
	}
	return nil
}

type Operation int

const (
	unknownOperation Operation = iota
	OperationGotLock
	OperationSeeksLock
	OperationUnlocks
)

func (l *lockfinder) report() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "report\n")
	for lock, fn := range l.m {
		if fn != "" {
			fmt.Fprintf(&buf, "- %s is held by %s\n", lock, fn)
		}
	}
	for lock, fn := range l.m {
		if fn == "" {
			fmt.Fprintf(&buf, "- %s is not held\n", lock)
		}
	}
	return buf.String()
}

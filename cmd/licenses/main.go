package main

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	cmd := exec.Command("go", "build", "./cmd/run")
	defer os.Remove("run")

	if err := cmd.Run(); err != nil {
		panic(err)
	}

	cmd = exec.Command("bash", "-c", `go version -m ./run | awk '$1=="dep" { print $2 "@" $3 }'`)
	var w strings.Builder
	cmd.Stdout = &w
	if err := cmd.Run(); err != nil {
		panic(err)
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	var out []string

	for _, dep := range strings.Split(w.String(), "\n") {
		depPathname, err := EncodePath(strings.TrimSpace(dep))
		if err != nil {
			panic(err)
		}
		if depPathname == "" {
			continue
		}
		fmt.Fprintln(os.Stderr, "used:", dep)
		dir := filepath.Join(gopath, "pkg", "mod", depPathname)

		fs, err := os.ReadDir(dir)
		if err != nil {
			panic(err)
		}

		for _, f := range fs {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if _, ok := licenseNames[name]; !ok {
				continue
			}
			bs, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				panic(err)
			}

			out = append(out, dep+"\n"+strings.Repeat("=", len(dep))+"\n\n"+string(bs))
			break
		}
	}

	if err := os.WriteFile(os.Args[len(os.Args)-1], []byte(strings.Join(out, "\n\n\n")), 0644); err != nil {
		panic(err)
	}
}

var licenseNames map[string]struct{}

func init() {
	names := strings.Split("COPYING, COPYING.md, COPYING.markdown, COPYING.txt, LICENCE, LICENCE.md, LICENCE.markdown, LICENCE.txt, LICENSE, LICENSE.md, LICENSE.markdown, LICENSE.txt, LICENSE-2.0.txt, LICENCE-2.0.txt, LICENSE-APACHE, LICENCE-APACHE, LICENSE-APACHE-2.0.txt, LICENCE-APACHE-2.0.txt, LICENSE-MIT, LICENCE-MIT, LICENSE.MIT, LICENCE.MIT, LICENSE.code, LICENCE.code, LICENSE.docs, LICENCE.docs, LICENSE.rst, LICENCE.rst, MIT-LICENSE, MIT-LICENCE, MIT-LICENSE.md, MIT-LICENCE.md, MIT-LICENSE.markdown, MIT-LICENCE.markdown, MIT-LICENSE.txt, MIT-LICENCE.txt, MIT_LICENSE, MIT_LICENCE, UNLICENSE, UNLICENCE", ", ")
	licenseNames = make(map[string]struct{}, len(names))
	for _, n := range names {
		licenseNames[n] = struct{}{}
	}
}

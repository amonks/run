package main

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path"
	"strings"

	"golang.org/x/mod/modfile"
)

func main() {
	bs, err := os.ReadFile("go.mod")
	if err != nil {
		panic(err)
	}

	f, err := modfile.Parse("go.mod", bs, nil)
	if err != nil {
		panic(err)
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	var out []string

	for _, r := range f.Require {
		if used, err := isUsed(r.Mod.Path); err != nil {
			panic(err)
		} else if !used {
			continue
		}
		fmt.Fprintln(os.Stderr, "used:", r.Mod.Path)
		modpath, err := EncodePath(r.Mod.String())
		if err != nil {
			panic(err)
		}
		dir := path.Join(gopath, "pkg/mod", modpath)

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
			bs, err := os.ReadFile(path.Join(dir, name))
			if err != nil {
				panic(err)
			}

			out = append(out, r.Mod.String()+"\n"+strings.Repeat("=", len(r.Mod.String()))+"\n\n"+string(bs))
			break
		}
	}

	if err := os.WriteFile(os.Args[len(os.Args)-1], []byte(strings.Join(out, "\n\n\n")), 0644); err != nil {
		panic(err)
	}
}

func isUsed(path string) (bool, error) {
	cmd := exec.Command("go", "mod", "why", "-m", path)
	w := &strings.Builder{}
	cmd.Stdout = w
	if err := cmd.Run(); err != nil {
		return false, err
	}
	for _, l := range strings.Split(w.String(), "\n") {
		if _, ok := notProd[l]; ok {
			return false, nil
		}
	}
	return true, nil
}

var notProd map[string]struct{}

func init() {
	devDeps := []string{
		"github.com/goreleaser/goreleaser",
		"golang.org/x/mod",
		"golang.org/x/tools",
		"github.com/sergi/go-diff",
	}
	notProd = make(map[string]struct{}, len(devDeps))
	for _, m := range devDeps {
		notProd[m] = struct{}{}
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

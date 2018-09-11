/*
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Command deadleaves finds and prints the import paths of unused Go packages.
// A package is considered unused if it is not a command ("package main") and
// is not transitively imported by a command.
package main

import (
	"flag"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
)

var stdFlag = flag.Bool("std", false, "report unused standard packages")
var installed = flag.Bool("installed", false, "only use installed binaries to check")
var wholeGit = flag.Bool("git", false, "only report whole unused git trees")

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findGit(pkg *build.Package) string {
	start := pkg.Dir
	end := pkg.SrcRoot
	if len(end) == 0 {
		return ""
	}
	curr := start
	for curr != end {
		dotGit := path.Join(curr, ".git")
		if exists(dotGit) {
			return curr
		}
		curr = path.Dir(curr)
	}
	return ""
}

func main() {
	ctx := build.Default

	flag.Parse()

	gits := make(map[string]string)
	pkgs := make(map[string]*build.Package)
	gitUsed := make(map[string]bool)
	for _, root := range ctx.SrcDirs() {
		err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if !fi.IsDir() {
				return nil
			}
			pkg, err := ctx.ImportDir(path, 0)
			if err != nil {
				return nil
			}
			pkgs[pkg.ImportPath] = pkg
			if *wholeGit {
				if g := findGit(pkg); g != "" {
					gits[pkg.ImportPath] = g
					gitUsed[g] = false

				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error walking %q: %v\n", root, err)
		}
	}

	used := make(map[string]bool)
	var recordDeps func(*build.Package)
	recordDeps = func(pkg *build.Package) {
		if used[pkg.ImportPath] {
			return
		}
		used[pkg.ImportPath] = true
		git := gits[pkg.ImportPath]
		if len(git) > 0 {
			gitUsed[git] = true
		}
		imports := append([]string{}, pkg.Imports...)
		imports = append(imports, pkg.TestImports...)
		for _, p := range imports {
			dep, err := ctx.Import(p, pkg.Dir, 0)
			if err != nil {
				if p != "C" {
					fmt.Fprintf(os.Stderr, "package %q not found (imported by %q)\n", p, pkg.ImportPath)
				}
				continue
			}
			recordDeps(dep)
		}
	}
	for _, pkg := range pkgs {
		if pkg.Name == "main" {
			if *installed {
				b := path.Base(pkg.Dir)
				bin := path.Join(pkg.BinDir, b)
				if !exists(bin) {
					fmt.Fprintf(os.Stderr, "Skipping %q, not installed\n", b)
					continue
				}
			}
			recordDeps(pkg)
		}
	}

	if *wholeGit {
		for path, used := range gitUsed {
			if used {
				continue
			}
			fmt.Println(path)
		}
	} else {
		for path, pkg := range pkgs {
			if !used[path] {
				if pkg.Goroot && !*stdFlag {
					continue
				}
				fmt.Println(path)
			}
		}
	}
}

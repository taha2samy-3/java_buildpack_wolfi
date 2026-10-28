package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type imageFlags []string

func (i *imageFlags) String() string     { return strings.Join(*i, ",") }
func (i *imageFlags) Set(v string) error { *i = append(*i, v); return nil }

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: site data|notes [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "data":
		fs := flag.NewFlagSet("data", flag.ExitOnError)
		repo := fs.String("repository", "taha2samy-3/java_buildpack_wolfi", "GitHub repository owner/name")
		out := fs.String("out", "site/src/data", "output directory")
		root := fs.String("root", ".", "repository root")
		releases := fs.String("releases", "", "read releases from a JSON file instead of the GitHub API")
		offline := fs.Bool("offline", false, "do not contact the GitHub API")
		_ = fs.Parse(os.Args[2:])
		err = Export(Options{Root: *root, Out: *out, Repository: *repo, ReleasesFile: *releases, Offline: *offline, Token: os.Getenv("GITHUB_TOKEN")})
	case "notes":
		fs := flag.NewFlagSet("notes", flag.ExitOnError)
		version := fs.String("version", "", "release version without the leading v")
		repo := fs.String("repository", "taha2samy-3/java_buildpack_wolfi", "GitHub repository owner/name")
		root := fs.String("root", ".", "repository root")
		var images imageFlags
		fs.Var(&images, "image", "name=image@digest, repeatable")
		_ = fs.Parse(os.Args[2:])
		err = Notes(os.Stdout, *root, *repo, *version, images)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

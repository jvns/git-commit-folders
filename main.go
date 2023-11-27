package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/jvns/git-commit-folders/fuse"
	"github.com/jvns/git-commit-folders/fuse2dav"
	"golang.org/x/net/webdav"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	repo, err := git.PlainOpen(".")
	if err != nil {
		log.Fatal(err)
	}

	mountpoint := flag.Arg(0)
	typ := "dav"
	if typ == "dav" {
		fs := fuse.New(repo)
		davFS := fuse2dav.Fuse2Dav(fs)
		srv := &webdav.Handler{
			FileSystem: davFS,
			LockSystem: webdav.NewMemLS(),
			Logger: func(r *http.Request, err error) {
				if err != nil {
					log.Printf("WEBDAV [%s]: %s, ERROR: %s\n", r.Method, r.URL, err)
				} else {
					log.Printf("WEBDAV [%s]: %s \n", r.Method, r.URL)
				}
			},
		}
		http.Handle("/", srv)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", 8999), nil); err != nil {
			log.Fatalf("Error with WebDAV server: %v", err)
		}
	} else {
		fuse.Run(repo, mountpoint)
	}
}

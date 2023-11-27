package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/jvns/git-commit-folders/fuse"
	"github.com/jvns/git-commit-folders/fuse2nfs"
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
		davFS := fuse2nfs.Fuse2Dav(fs)
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
		///* mount mountpoint*/
		//cmd := exec.Command("mount", "-t", "webdav", "-o", "rw", "http://localhost:8999", mountpoint)
		//cmd.Stdout = os.Stdout
		//cmd.Stderr = os.Stderr
		//if err := cmd.Run(); err != nil {
		//    log.Fatalf("Error mounting %s: %v", mountpoint, err)
		//}

		if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", 8999), nil); err != nil {
			log.Fatalf("Error with WebDAV server: %v", err)
		}
	} else if typ == "nfs" {
		fs := fuse.New(repo)
		nfsFS := fuse2nfs.Fuse2NFS(fs)
		fuse2nfs.RunServer(nfsFS, 8999)
	} else {
		fuse.Run(repo, mountpoint)
	}
}

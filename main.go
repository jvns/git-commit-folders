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

type options struct {
	typ        string
	mountpoint string
	repoDir    string
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.typ, "type", "webdav", "type of mount (webdav, nfs, or fuse)")
	flag.StringVar(&opts.mountpoint, "mountpoint", "", "mountpoint")
	flag.StringVar(&opts.repoDir, "repo", ".", "repo dir")
	flag.Parse()
	if opts.typ != "webdav" && opts.typ != "nfs" && opts.typ != "fuse" {
		usage()
		log.Fatalf("Invalid type %s\n", opts.typ)
	}
	return opts
}

func main() {
	opts := parseOptions()
	repo, err := git.PlainOpen(opts.repoDir)
	if err != nil {
		log.Fatal(err)
	}

	mountpoint := flag.Arg(0)
	if opts.typ == "webdav" {
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
	} else if opts.typ == "nfs" {
		fs := fuse.New(repo)
		nfsFS := fuse2nfs.Fuse2NFS(fs)
		fuse2nfs.RunServer(nfsFS, 8999)
	} else {
		fuse.Run(repo, mountpoint)
	}
}

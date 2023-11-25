package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/anacrolix/fuse/fs/fstestutil"
	git "github.com/go-git/go-git/v5"
	"github.com/jvns/git-commit-folders/fuse"
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
	fuse.Run(repo, mountpoint)
}

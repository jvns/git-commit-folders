package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	_ "github.com/anacrolix/fuse/fs/fstestutil"
	git "github.com/go-git/go-git/v5"
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
	mountpoint := flag.Arg(0)

	defer fuse.Unmount(mountpoint) // TODO: doesn't seem to work
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("helloworld"),
		fuse.Subtype("hellofs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Hello world!"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	repo, err := git.PlainOpen(".")

	err = fs.Serve(c, &FS{repo})
	if err != nil {
		log.Fatal(err)
	}

	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

// FS implements the hello world file system.
type FS struct {
	repo *git.Repository
}

func (f *FS) Root() (fs.Node, error) {
	return f, nil
}

func (f *FS) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	return nil
}

func (f *FS) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{
		{Name: "commits", Type: fuse.DT_Dir},
		{Name: "branches", Type: fuse.DT_Dir},
		{Name: "branch_histories", Type: fuse.DT_Dir},
	}, nil
}

func (f *FS) Lookup(ctx context.Context, name string) (fs.Node, error) {
	switch name {
	case "commits":
		return &CommitsDir{repo: f.repo}, nil
	case "branches":
		return &BranchesDir{repo: f.repo}, nil
	case "branch_histories":
		return &BranchHistoriesDir{repo: f.repo}, nil
	}
	return nil, fuse.ENOENT
}

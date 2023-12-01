package fuse

import (
	"context"
	"log"
	"os"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	_ "github.com/anacrolix/fuse/fs/fstestutil"
	git "github.com/go-git/go-git/v5"
)

func Run(repo *git.Repository, mountpoint string) {
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

func New(repo *git.Repository) *FS {
	return &FS{repo: repo}
}

func (f *FS) Root() (fs.Node, error) {
	return f, nil
}

func (f *FS) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Inode = 1
	return nil
}

func (f *FS) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{
		{Name: "commits", Type: fuse.DT_Dir},
		{Name: "branches", Type: fuse.DT_Dir},
		{Name: "tags", Type: fuse.DT_Dir},
		{Name: "branch_histories", Type: fuse.DT_Dir},
	}, nil
}

func (f *FS) Lookup(ctx context.Context, name string) (fs.Node, error) {
	switch name {
	case "commits":
		return &CommitsDir{repo: f.repo}, nil
	case "branches":
		return &BranchesDir{repo: f.repo}, nil
	case "tags":
		return &TagsDir{repo: f.repo}, nil
	case "branch_histories":
		return &BranchHistoriesDir{repo: f.repo}, nil
	}
	return nil, fuse.ENOENT
}

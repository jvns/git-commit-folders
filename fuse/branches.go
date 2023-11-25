package fuse

import (
	"context"
	"os"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type BranchesDir struct {
	repo *git.Repository
}

func (f *BranchesDir) Root() (fs.Node, error) {
	return f, nil
}

func (f *BranchesDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	return nil
}

func (f *BranchesDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	branches, err := f.repo.Branches()
	if err != nil {
		return nil, err
	}
	branches.ForEach(func(branch *plumbing.Reference) error {
		entries = append(entries, fuse.Dirent{
			Name: branch.Name().Short(),
			Type: fuse.DT_Link,
		})
		return nil
	})
	return entries, nil
}

func (f *BranchesDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	ref, err := f.repo.Reference(plumbing.ReferenceName("refs/heads/"+name), true)
	if err != nil {
		return nil, fuse.ENOENT
	}
	/* return a symlink to ../commits/<hash> */
	id := ref.Hash().String()
	return &SymLink{"../commits/" + id}, nil
}

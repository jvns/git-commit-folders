package main

import (
	"context"
	"os"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type TagsDir struct {
	repo *git.Repository
}

func (f *TagsDir) Root() (fs.Node, error) {
	return f, nil
}

func (f *TagsDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	return nil
}

func (f *TagsDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	tags, err := f.repo.Tags()
	if err != nil {
		return nil, err
	}
	tags.ForEach(func(branch *plumbing.Reference) error {
		entries = append(entries, fuse.Dirent{
			Name: branch.Name().Short(),
			Type: fuse.DT_Link,
		})
		return nil
	})
	return entries, nil
}

func (f *TagsDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	refName := plumbing.ReferenceName("refs/tags/" + name)
	// we need to resolve the reference in case it's symbolic
	// TODO: this doesn't seem to work for the tags in git's own repo
	ref, err := f.repo.Reference(refName, true)
	if err != nil {
		return nil, fuse.ENOENT
	}
	id := ref.Hash().String()
	return &SymLink{"../commits/" + id}, nil
}

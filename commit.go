package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	_ "github.com/anacrolix/fuse/fs/fstestutil"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
)

type GitTree struct {
	repo *git.Repository
	id   plumbing.Hash
}

type GitBlob struct {
	repo *git.Repository
	id   plumbing.Hash
}

func (t *GitTree) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = NEXT_ID.Add(1)
	a.Mode = os.ModeDir | 0o555
	return nil
}

func (t *GitTree) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Printf("Lookup %s\n", name)
	tree, err := t.repo.TreeObject(t.id)
	if err != nil {
		return nil, fmt.Errorf("lookup %s: %w", name, err)
	}

	for _, entry := range tree.Entries {
		if entry.Name == name {
			switch entry.Mode {
			case filemode.Dir:
				return &GitTree{id: entry.Hash}, nil
			case filemode.Regular:
				return &GitBlob{id: entry.Hash}, nil
			}
		}
	}
	return nil, fuse.ENOENT
}

func (b *GitTree) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	tree, err := b.repo.TreeObject(b.id)
	if err != nil {
		return nil, err
	}

	var dirs []fuse.Dirent
	for _, entry := range tree.Entries {
		var d fuse.Dirent
		switch entry.Mode {
		case filemode.Dir:
			d.Type = fuse.DT_Dir
		case filemode.Regular:
			d.Type = fuse.DT_File
		}
		d.Name = entry.Name
		d.Inode = NEXT_ID.Add(1)
		dirs = append(dirs, d)
	}
	return dirs, nil
}

func (b *GitBlob) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = NEXT_ID.Add(1)
	a.Mode = 0o444
	return nil
}

func (b *GitBlob) ReadAll(ctx context.Context) ([]byte, error) {
	blob, err := b.repo.BlobObject(b.id)
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}
	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

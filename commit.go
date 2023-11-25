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

type CommitsDir struct {
	repo *git.Repository
}

func (f *CommitsDir) Root() (fs.Node, error) {
	return f, nil
}

func (f *CommitsDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	return nil
}

func (f *CommitsDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	rev, err := f.repo.ResolveRevision(plumbing.Revision(name))
	if err != nil {
		return nil, fuse.ENOENT
	}
	/* if it's a prefix, symlink to the full name */
	if rev.String() != name {
		return &SymLink{content: rev.String()}, nil
	}
	commit, err := f.repo.CommitObject(*rev)
	if err != nil {
		return nil, fmt.Errorf("error getting commit %s: %w", name, err)
	}
	return &GitTree{repo: f.repo, id: commit.TreeHash}, nil
}

type GitTree struct {
	repo *git.Repository
	id   plumbing.Hash
}

type GitBlob struct {
	repo *git.Repository
	id   plumbing.Hash
}

func (t *GitTree) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	return nil
}

func (t *GitTree) Lookup(ctx context.Context, name string) (fs.Node, error) {
	tree, err := t.repo.TreeObject(t.id)
	if err != nil {
		return nil, fmt.Errorf("lookup %s: %w", name, err)
	}

	for _, entry := range tree.Entries {
		if entry.Name == name {
			switch entry.Mode {
			case filemode.Dir:
				return &GitTree{repo: t.repo, id: entry.Hash}, nil
			case filemode.Regular:
				return &GitBlob{repo: t.repo, id: entry.Hash}, nil
			default:
				fmt.Printf("Unknown mode %s\n", entry.Mode)
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
		dirs = append(dirs, d)
	}
	return dirs, nil
}

func (b *GitBlob) Attr(ctx context.Context, a *fuse.Attr) error {
	content, err := b.ReadAll(ctx)
	if err != nil {
		return err
	}
	a.Mode = 0o444
	a.Size = uint64(len(content))
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

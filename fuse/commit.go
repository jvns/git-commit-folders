package fuse

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

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

type CommitsPrefixDir struct {
	repo   *git.Repository
	prefix string
}

func (f *CommitsDir) Root() (fs.Node, error) {
	return f, nil
}

func (f *CommitsDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Mtime = time.Unix(0, 0)
	a.Ctime = time.Unix(0, 0)
	a.Inode = inode("/commits")
	return nil
}

type CommitsCache struct {
	commits map[string][]string
	expiry  time.Time
}

var cachedCommits *CommitsCache

func getCommits(repo *git.Repository) (map[string][]string, error) {
	if cachedCommits == nil || cachedCommits.expiry.Before(time.Now()) {
		objStorer := repo.Storer
		commits := make(map[string][]string)
		iter, err := objStorer.IterEncodedObjects(plumbing.CommitObject)
		if err != nil {
			return nil, err
		}
		start := time.Now()
		for {
			commit, err := iter.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			commits[commit.Hash().String()[:2]] = append(commits[commit.Hash().String()[:2]], commit.Hash().String())
		}
		elapsed := time.Since(start)
		// cache for 20x the time it took to read the commits
		cacheDuration := elapsed * 20
		if cacheDuration > 1*time.Minute {
			cacheDuration = 1 * time.Minute
		}
		cachedCommits = &CommitsCache{commits, time.Now().Add(cacheDuration)}
	}
	return cachedCommits.commits, nil
}

func (f *CommitsDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	commits, err := getCommits(f.repo)
	if err != nil {
		return nil, err
	}
	var entries []fuse.Dirent
	for prefix, _ := range commits {
		entries = append(entries, fuse.Dirent{
			Name: prefix,
			Type: fuse.DT_Dir,
		})
	}
	return entries, nil
}

func (f *CommitsDir) Lookup(ctx context.Context, prefix string) (fs.Node, error) {
	return &CommitsPrefixDir{repo: f.repo, prefix: prefix}, nil
}

func (f *CommitsPrefixDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Inode = inode("/commits/" + f.prefix)
	return nil
}

func (f *CommitsPrefixDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	commits, err := getCommits(f.repo)
	if err != nil {
		return nil, err
	}
	entries := []fuse.Dirent{}
	for _, commit := range commits[f.prefix] {
		entries = append(entries, fuse.Dirent{
			Name: commit,
			Type: fuse.DT_Dir,
		})
	}
	return entries, nil
}

func (f *CommitsPrefixDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	/* get the git tree */
	commit, err := f.repo.CommitObject(plumbing.NewHash(name))
	if err != nil {
		return nil, fuse.ENOENT
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
	mode filemode.FileMode
}

func (t *GitTree) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Inode = inode(t.id.String())
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
				return &GitBlob{repo: t.repo, id: entry.Hash, mode: entry.Mode}, nil
			case filemode.Executable:
				return &GitBlob{repo: t.repo, id: entry.Hash, mode: entry.Mode}, nil
			case filemode.Symlink:
				content, err := readBlob(t.repo, entry.Hash)
				if err != nil {
					return nil, fmt.Errorf("read symlink: %w", err)
				}
				return &SymLink{string(content)}, nil
			case filemode.Submodule:
				fmt.Printf("warning: submodule %s not supported\n", entry.Name)
				return nil, fuse.ENOENT
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
		case filemode.Executable:
			d.Type = fuse.DT_File
		case filemode.Symlink:
			d.Type = fuse.DT_Link
		default:
			fmt.Printf("%s has unknown mode %s\n", entry.Name, entry.Mode)
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
	switch b.mode {
	case filemode.Executable:
		a.Mode = 0o555
	case filemode.Symlink:
		a.Mode = os.ModeSymlink | 0o555
	default:
		a.Mode = 0o444
	}
	a.Size = uint64(len(content))
	a.Mtime = time.Unix(0, 0)
	a.Ctime = time.Unix(0, 0)
	return nil
}

func readBlob(repo *git.Repository, id plumbing.Hash) ([]byte, error) {
	blob, err := repo.BlobObject(id)
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

func (b *GitBlob) ReadAll(ctx context.Context) ([]byte, error) {
	return readBlob(b.repo, b.id)
}

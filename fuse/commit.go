package fuse

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	_ "github.com/anacrolix/fuse/fs/fstestutil"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type CommitsDir struct {
	repo *git.Repository
}

type CommitsPrefixDir struct {
	/* /commits/af */
	repo   *git.Repository
	prefix string
}

type CommitsPrefixDir2 struct {
	/* /commits/af/afee */
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

/*
  2-level map: 47e33c05f9f07cac3de833e531bcac9ae052c7c is stored as
  commits["47"]["47e3"]["47e33c05f9f07cac3de833e531bcac9ae052c7c"] = true

  it's 2 levels so that we can handle repos with 1 million commits without
  making listing commits unbearably slow. Otherwise `ls` is just a disaster.
*/

type CommitsCache struct {
	commits map[string]map[string]map[string]bool
	expiry  time.Time
}

var cachedCommits *CommitsCache

func addToCache(item plumbing.Hash) error {
	id := item.String()
	prefix1 := id[:2]
	prefix2 := id[:4]
	if _, ok := cachedCommits.commits[prefix1]; !ok {
		cachedCommits.commits[prefix1] = make(map[string]map[string]bool)
	}
	if _, ok := cachedCommits.commits[prefix1][prefix2]; !ok {
		cachedCommits.commits[prefix1][prefix2] = make(map[string]bool)
	}
	cachedCommits.commits[prefix1][prefix2][id] = true
	return nil
}

/*
Just iterate over the full repo once at the beginning (called in `root.go`), otherwise only look at
the loose objects.

This assumes two false things:
* repos never get repacked
* commits never get deleted

hopefully they're true enough most of the time though
*/
func getPackedCommits(repo *git.Repository) error {
	if cachedCommits != nil {
		return nil
	}
	objStorer := repo.Storer
	iter, err := objStorer.IterEncodedObjects(plumbing.CommitObject)
	if err != nil {
		return err
	}
	cachedCommits = &CommitsCache{commits: make(map[string]map[string]map[string]bool)}
	iter.ForEach(func(obj plumbing.EncodedObject) error {
		return addToCache(obj.Hash())
	})
	log.Printf("Done caching packed commits")
	return nil
}

func getCommits(repo *git.Repository) (map[string]map[string]map[string]bool, error) {
	getPackedCommits(repo)
	if cachedCommits.expiry.Before(time.Now()) {
		if los, ok := repo.Storer.(storer.LooseObjectStorer); ok {
			start := time.Now()
			los.ForEachObjectHash(func(hash plumbing.Hash) error {
				commit, err := repo.CommitObject(hash)
				if err != nil {
					return nil
				}
				addToCache(commit.Hash)
				return nil
			})
			elapsed := time.Since(start)
			cacheDuration := elapsed * 20
			if cacheDuration > 1*time.Minute {
				cacheDuration = 1 * time.Minute
			}
			cachedCommits.expiry = time.Now().Add(cacheDuration)
		} else {
			log.Fatal("can't get loose objects")
		}
	}
	return cachedCommits.commits, nil
}

func (f *CommitsDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	commits, err := getCommits(f.repo)
	if err != nil {
		log.Printf("error: can't get commits: %v", err)
		return nil, err
	}
	var entries []fuse.Dirent
	for prefix := range commits {
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
		log.Printf("error: can't get commits: %v", err)
		return nil, err
	}
	var entries []fuse.Dirent
	for prefix := range commits[f.prefix] {
		entries = append(entries, fuse.Dirent{
			Name: prefix,
			Type: fuse.DT_Dir,
		})
	}
	return entries, nil
}

func (f *CommitsPrefixDir) Lookup(ctx context.Context, prefix string) (fs.Node, error) {
	return &CommitsPrefixDir2{repo: f.repo, prefix: prefix}, nil
}

func (f *CommitsPrefixDir2) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Inode = inode("/commits/" + f.prefix[:2] + "/" + f.prefix)
	return nil
}

func (f *CommitsPrefixDir2) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	commits, err := getCommits(f.repo)
	if err != nil {
		log.Printf("error: can't get commits: %v", err)
		return nil, err
	}
	entries := []fuse.Dirent{}
	for commit := range commits[f.prefix[:2]][f.prefix] {
		entries = append(entries, fuse.Dirent{
			Name: commit,
			Type: fuse.DT_Dir,
		})
	}
	return entries, nil
}

func (f *CommitsPrefixDir2) Lookup(ctx context.Context, name string) (fs.Node, error) {
	/* get the git tree */
	commit, err := f.repo.CommitObject(plumbing.NewHash(name))
	if err != nil {
		log.Printf("error: can't get commit object: %v", err)
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
		log.Printf("error: can't read tree object: %v", err)
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
			// TODO: this is a weird thing to do but I don't feel like dealing
			// with submodules and otherwise things break
			fmt.Printf("%s has unknown mode %s, skipping\n", entry.Name, entry.Mode)
			continue
		}
		d.Name = entry.Name
		dirs = append(dirs, d)
	}
	return dirs, nil
}

func (b *GitBlob) Attr(ctx context.Context, a *fuse.Attr) error {
	content, err := b.ReadAll(ctx)
	if err != nil {
		log.Printf("error: can't read git blob: %v", err)
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

func commitPath(id string) string {
	return "commits/" + id[:2] + "/" + id[:4] + "/" + id
}

package fuse

import (
	"context"
	"fmt"
	"os"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type BranchHistoriesDir struct {
	repo *git.Repository
}

type BranchHistoryDir struct {
	repo   *git.Repository
	branch string
}

func (f *BranchHistoriesDir) Root() (fs.Node, error) {
	return f, nil
}

func (f *BranchHistoriesDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Inode = inode("/branch_histories")
	return nil
}

func (f *BranchHistoriesDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	branches, err := f.repo.Branches()
	if err != nil {
		return nil, err
	}
	branches.ForEach(func(branch *plumbing.Reference) error {
		entries = append(entries, fuse.Dirent{
			Name: branch.Name().Short(),
			Type: fuse.DT_Dir,
		})
		return nil
	})
	return entries, nil
}

func (f *BranchHistoriesDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	/* make sure branch exists */
	_, err := f.repo.Reference(plumbing.ReferenceName("refs/heads/"+name), true)
	if err != nil {
		return nil, fuse.ENOENT
	}
	return &BranchHistoryDir{repo: f.repo, branch: name}, nil
}

func (f *BranchHistoryDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0o555
	a.Inode = inode("/branch_histories/" + f.branch)
	return nil
}

func (f *BranchHistoryDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	// todo: maybe make this configurable
	MAX_COMMITS := 100
	/* list last 20 commits, like 00-ID, symlink to ../commits/ID */
	var entries []fuse.Dirent
	ref, err := f.repo.Reference(plumbing.ReferenceName("refs/heads/"+f.branch), true)
	commits, err := f.repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}
	var i int
	for {
		commit, err := commits.Next()
		if err != nil {
			break
		}
		if i >= MAX_COMMITS {
			break
		}
		entries = append(entries, fuse.Dirent{
			Name: fmt.Sprintf("%02d-%s", i, commit.Hash.String()),
			Type: fuse.DT_Link,
		})
		i++
	}
	return entries, nil
}

func (f *BranchHistoryDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	/* extract commit hash from name */
	hash := name[3:]
	_, err := f.repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, fuse.ENOENT
	}
	return &SymLink{"../../" + commitPath(hash)}, nil
}

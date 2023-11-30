package fuse

import (
	"errors"

	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
	"github.com/go-git/go-git/v5/utils/ioutil"
)

func path(repo *git.Repository) (string, error) {
	s, ok := repo.Storer.(*filesystem.Storage)
	if !ok {
		return "", errors.New("Repository storage is not filesystem.Storage")
	}
	fs, ok := s.Filesystem().(*chroot.ChrootHelper)
	if !ok {
		return "", errors.New("Filesystem is not chroot.ChrootHelper")
	}

	return fs.Root(), nil
}

func getDotGit(repo *git.Repository) (*dotgit.DotGit, error) {
	root, err := path(repo)
	if err != nil {
		return nil, err
	}
	return dotgit.New(osfs.New(root)), nil
}

func getIndexes(repo *git.Repository) ([]*idxfile.MemoryIndex, error) {
	dotgit, err := getDotGit(repo)
	if err != nil {
		return nil, err
	}
	packs, err := dotgit.ObjectPacks()
	if err != nil {
		return nil, err
	}
	var indexes []*idxfile.MemoryIndex
	for _, packHash := range packs {
		f, err := dotgit.ObjectPackIdx(packHash)
		if err != nil {
			return nil, err
		}

		defer ioutil.CheckClose(f, &err)

		idxf := idxfile.NewMemoryIndex()
		d := idxfile.NewDecoder(f)
		if err = d.Decode(idxf); err != nil {
			return nil, err
		}
		indexes = append(indexes, idxf)
	}
	return indexes, nil
}

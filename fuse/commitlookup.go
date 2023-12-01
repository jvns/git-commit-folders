package fuse

import (
	"encoding/hex"
	"errors"
	"io"

	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
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

func twoDigitPrefixes(repo *git.Repository) ([]string, error) {
	s, ok := repo.Storer.(*filesystem.Storage)
	if !ok {
		return nil, errors.New("Repository storage is not filesystem.Storage")
	}
	var prefixes []string
	for i := 0; i < 256; i++ {
		prefix := []byte{byte(i)}
		iter, err := s.IterEncodedObjectsPrefix(plumbing.CommitObject, prefix)
		obj, err := iter.Next()
		if err == io.EOF {
			continue
		}
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, hex.EncodeToString(prefix))
	}
	return prefixes, nil
}

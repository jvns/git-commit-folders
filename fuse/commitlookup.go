package fuse

import (
	"bytes"
	"errors"

	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/hash"
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

func getIndexes(repo *git.Repository) ([]*PackedIndex, error) {
	dotgit, err := getDotGit(repo)
	if err != nil {
		return nil, err
	}
	packs, err := dotgit.ObjectPacks()
	if err != nil {
		return nil, err
	}
	var indexes []*PackedIndex
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
		indexes = append(indexes, &PackedIndex{*idxf})
	}
	return indexes, nil
}

func initCache(repo *git.Repository) (*Cache, error) {
	indexes, err := getIndexes(repo)
	if err != nil {
		return nil, err
	}
	root, _ := path(repo)
	return &Cache{indexes: indexes, repo: repo, los: LooseObjectStore{osfs.New(root)}}, nil
}

type Cache struct {
	indexes []*PackedIndex
	repo    *git.Repository
	los     LooseObjectStore
}

func (c *Cache) ObjectStores() []ObjectStore {
	var stores []ObjectStore
	for _, index := range c.indexes {
		stores = append(stores, index)
	}
	stores = append(stores, &c.los)
	return stores
}

func (c *Cache) HasPrefix(prefix string) bool {
	stores := c.ObjectStores()
	for _, store := range stores {
		if store.HasPrefix(plumbing.NewHash(prefix)) {
			return true
		}
	}
	return false
}

func (c *Cache) TwoDigitPrefixes() ([]string, error) {
	prefixes := make(map[string]bool)
	for _, store := range c.ObjectStores() {
		twoDigitPrefixes, err := store.TwoDigitPrefixes()
		if err != nil {
			return nil, err
		}
		for _, prefix := range twoDigitPrefixes {
			prefixes[prefix] = true
		}
	}
	var prefixesSlice []string
	for prefix := range prefixes {
		prefixesSlice = append(prefixesSlice, prefix)
	}
	return prefixesSlice, nil
}

func findHashIndex(idx *PackedIndex, h plumbing.Hash) (int, bool) {
	objectIDLength := uint64(hash.Size)
	noMapping := -1
	k := idx.FanoutMapping[h[0]]
	if k == noMapping {
		return 0, false
	}

	if len(idx.Names) <= k {
		return 0, false
	}

	data := idx.Names[k]
	high := uint64(len(idx.Offset32[k])) >> 2
	if high == 0 {
		return 0, false
	}

	low := uint64(0)
	for {
		mid := (low + high) >> 1
		offset := mid * objectIDLength

		cmp := bytes.Compare(h[:], data[offset:offset+objectIDLength])
		if cmp < 0 {
			high = mid
		} else if cmp == 0 {
			return int(mid), true
		} else {
			low = mid + 1
		}

		if low >= high {
			break
		}
	}

	return 0, false
}

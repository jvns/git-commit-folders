package fuse

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/go-git/go-billy"
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

func initCache(repo *git.Repository) (*Cache, error) {
	indexes, err := getIndexes(repo)
	if err != nil {
		return nil, err
	}
	return &Cache{indexes: indexes, repo: repo}, nil
}

type Cache struct {
	indexes []*idxfile.MemoryIndex
	repo    *git.Repository
	fs      billy.Filesystem
}

func IndexHasPrefix(index *idxfile.MemoryIndex, hash plumbing.Hash) bool {
	_, found := findHashIndex(index, hash)
	return found
}

func LooseObjectsHasPrefix(fs billy.Filesystem, hash plumbing.Hash) bool {
	sha := hash.String()
	dir, err := fs.ReadDir(fs.Join("objects", sha[:2]))
	if err != nil {
		return false
	}
	for _, entry := range dir {
		name := entry.Name()
		if strings.HasPrefix(name, sha[2:]) {
			return true
		}
	}
	return false
}

func (c *Cache) HasPrefix(prefix string) bool {
	prefixHash := plumbing.NewHash(prefix)
	for _, index := range c.indexes {
		if IndexHasPrefix(index, prefixHash) {
			return true
		}
	}
	return LooseObjectsHasPrefix(c.fs, prefixHash)
}

func (c *Cache) LooseTwoDigitPrefixes() []string {
	var prefixes []string
	dir, err := c.fs.ReadDir("objects")
	if err != nil {
		return nil
	}
	for _, entry := range dir {
		name := entry.Name()
		if len(name) != 2 {
			continue
		}
		prefixes = append(prefixes, name)
	}
	return prefixes
}

func IndexTwoDigitPrefixes(idx *idxfile.MemoryIndex) []string {
	noMapping := -1
	var prefixes []string
	for i := 0; i < 256; i++ {
		k := idx.FanoutMapping[i]
		if k != noMapping {
			/* convert i to hex */
			prefix := hex.EncodeToString([]byte{byte(i)})
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

func (c *Cache) TwoDigitPrefixes() []string {
	var prefixSet map[string]bool
	for _, index := range c.indexes {
		for _, prefix := range IndexTwoDigitPrefixes(index) {
			prefixSet[prefix] = true
		}
	}
	for _, prefix := range c.LooseTwoDigitPrefixes() {
		prefixSet[prefix] = true
	}
	var prefixes []string
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

/* copied from idxfile.go */
func findHashIndex(idx *idxfile.MemoryIndex, h plumbing.Hash) (int, bool) {
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

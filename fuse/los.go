package fuse

import (
	"encoding/hex"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
)

type ObjectStore interface {
	ObjectsWithPrefix(prefix plumbing.Hash, typ plumbing.ObjectType) (storer.EncodedObjectIter, error)
}

type PackedIndex struct {
	idxfile.MemoryIndex
}

type LooseObjectStore struct {
	billy.Filesystem
}

func (idx *PackedIndex) ObjectsWithPrefix(prefix plumbing.Hash, typ plumbing.ObjectType) (storer.EncodedObjectIter, error) {
    index, err := findHashIndex(idx, prefix)
    if err != nil {
        return nil, err
    }
    for (index < len(idx.Names)) {
        name := idx.Names[index]
        if !strings.HasPrefix(name, prefix.String()) {
            break
        }
        obj := p.objectAtOffset(idx.Offsets[index], hash)
        index++
    }

}
func (idx *PackedIndex) TwoDigitPrefixes(typ plumbing.ObjectType) ([]string, error) {
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
	return prefixes, nil
}

func (idx *PackedIndex) HasPrefix(hash plumbing.Hash, typ plumbing.ObjectType) bool {
	_, found := findHashIndex(idx, hash)
	return found
}

func (los *LooseObjectStore) TwoDigitPrefixes(typ plumbing.ObjectType) ([]string, error) {
	var prefixes []string
	dir, err := los.ReadDir("objects")
	if err != nil {
		return nil, err
	}
	for _, entry := range dir {
		name := entry.Name()
		if len(name) != 2 {
			continue
		}
        dir, err = los.ReadDir(los.Join("objects", name))
        if err != nil {
            return nil, err
        }
        for _, entry := range dir {
            name := entry.Name()
            // check if it has the right type

		prefixes = append(prefixes, name)
	}
	return prefixes, nil
}

func (los *LooseObjectStore) HasPrefix(hash plumbing.Hash, typ plumbing.ObjectType) bool {
	sha := hash.String()
	dir, err := los.ReadDir(los.Join("objects", sha[:2]))
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

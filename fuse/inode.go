package fuse

import "hash/fnv"

/* generate inode by hashing filename */

func inode(filename string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(filename))
	return h.Sum64()
}

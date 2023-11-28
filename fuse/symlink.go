package fuse

import (
	"context"
	"os"
	"time"

	"github.com/anacrolix/fuse"
)

type SymLink struct {
	content string
}

func (s *SymLink) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeSymlink | 0o555
	a.Size = uint64(len(s.content))
	a.Mtime = time.Unix(0, 0)
	a.Ctime = time.Unix(0, 0)
	/* TODO: is it bad to define the inode this way? */
	a.Inode = inode(s.content)
	return nil
}

func (s *SymLink) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	return s.content, nil
}

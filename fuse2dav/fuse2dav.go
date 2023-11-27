package fuse2dav

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	"golang.org/x/net/webdav"
)

type FuseDavFS struct {
	fs fs.FS
}
type FuseDavNode struct {
	node      fs.Node
	path      string
	bytesRead int
	allBytes  []byte
	filesRead int
	allFiles  []os.FileInfo
}
type FuseAttr struct {
	attr fuse.Attr
	name string
}

func Fuse2Dav(fs fs.FS) webdav.FileSystem {
	return webdav.FileSystem(&FuseDavFS{fs: fs})
}

func (f *FuseDavFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return fmt.Errorf("Not implemented")
}

func findNode(ctx context.Context, root fs.FS, path string) (fs.Node, error) {
	lookedUp := []string{}
	node, err := root.Root()
	if err != nil {
		return nil, err
	}
	nameParts := strings.Split(path, "/")
	for _, part := range nameParts {
		if part == "" {
			continue
		}
		if n, ok := node.(fs.NodeStringLookuper); ok {
			node, err = n.Lookup(ctx, part)
			if err != nil {
				return nil, fmt.Errorf("Error looking up %s: %s", strings.Join(lookedUp, "/"), err)
			}
			lookedUp = append(lookedUp, part)
		} else {
			return nil, fmt.Errorf("Path %s does not implement NodeStringLookuper", strings.Join(lookedUp, "/"))
		}
	}
	return node, nil
}

/* 	OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (File, error) */
func (f *FuseDavFS) OpenFile(ctx context.Context, path string, flag int, perm os.FileMode) (webdav.File, error) {
	node, err := findNode(ctx, f.fs, path)
	if err != nil {
		return nil, err
	}
	return &FuseDavNode{node: node, path: path}, nil
}

func (f *FuseDavFS) RemoveAll(ctx context.Context, name string) error {
	return fmt.Errorf("Not implemented")
}

func (f *FuseDavFS) Rename(ctx context.Context, oldName, newName string) error {
	return fmt.Errorf("Not implemented")
}

func (f *FuseDavFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	node, err := findNode(ctx, f.fs, name)
	if err != nil {
		return nil, err
	}
	a := fuse.Attr{}
	err = node.Attr(ctx, &a)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(name, "/")
	return FuseAttr{attr: a, name: parts[len(parts)-1]}, nil
}

/* FileInfo implementation for FuseAttr */

func (f FuseAttr) Name() string {
	return f.name
}

func (f FuseAttr) Size() int64 {
	return int64(f.attr.Size)
}

func (f FuseAttr) Mode() os.FileMode {
	return f.attr.Mode
}

func (f FuseAttr) ModTime() time.Time {
	return f.attr.Mtime
}

func (f FuseAttr) IsDir() bool {
	return f.attr.Mode.IsDir()
}

func (f FuseAttr) Sys() interface{} {
	return nil
}

/* http.File implementation for FuseDavNode */

func (f *FuseDavNode) Close() error {
	return nil
}

func (f *FuseDavNode) ReadBytes() error {
	if f.allBytes != nil {
		return nil
	}
	if n, ok := f.node.(fs.HandleReadAller); ok {
		ctx := context.Background()
		var err error
		f.allBytes, err = n.ReadAll(ctx)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Node does not implement HandleReadAller")
	}
	return nil
}

func (f *FuseDavNode) Read(p []byte) (n int, err error) {
	err = f.ReadBytes()
	if err != nil {
		return 0, err
	}

	n = copy(p, f.allBytes[f.bytesRead:])
	f.bytesRead += n
	return n, nil
}

func (f *FuseDavNode) Seek(offset int64, whence int) (int64, error) {
	err := f.ReadBytes()
	if err != nil {
		return 0, err
	}
	switch whence {
	case io.SeekStart:
		f.bytesRead = int(offset)
	case io.SeekCurrent:
		f.bytesRead += int(offset)
	case io.SeekEnd:
		f.bytesRead = len(f.allBytes) + int(offset)
	}
	return int64(f.bytesRead), nil
}

func (f *FuseDavNode) Readdir(count int) ([]os.FileInfo, error) {
	node := f.node
	if f.allFiles == nil {
		if n, ok := node.(fs.HandleReadDirAller); ok {
			ctx := context.Background()
			nodes, err := n.ReadDirAll(ctx)
			if err != nil {
				return nil, err
			}
			attrs := []os.FileInfo{}
			for _, node := range nodes {
				attr := fuse.Attr{}
				switch node.Type {
				case fuse.DT_Dir:
					attr.Mode = os.ModeDir
				case fuse.DT_File:
					attr.Mode = 0644
				case fuse.DT_Link:
					attr.Mode = os.ModeSymlink
				default:
					attr.Mode = 0644
				}

				attrs = append(attrs, FuseAttr{attr: attr, name: node.Name})
			}
			f.allFiles = attrs
		} else {
			return []os.FileInfo{}, nil
		}
	}
	if count == 0 {
		count = len(f.allFiles)
	}
	infos := f.allFiles[f.filesRead : f.filesRead+count]
	f.filesRead += len(infos)
	return infos, nil
}

func (f *FuseDavNode) Stat() (os.FileInfo, error) {
	ctx := context.Background()
	attr := fuse.Attr{}
	err := f.node.Attr(ctx, &attr)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(f.path, "/")
	return FuseAttr{attr: attr, name: parts[len(parts)-1]}, nil
}

func (f *FuseDavNode) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("Not implemented")
}

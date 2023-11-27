package fuse2nfs

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	"golang.org/x/net/webdav"
)

type FuseDavFS struct {
	fs fs.FS
}

type FuseDavFile struct {
	node      fs.Node
	name      string
	bytesRead int
	allBytes  []byte
	filesRead int
	allFiles  []os.FileInfo
}

func (f *FuseDavFile) ToNFSFile() *FuseFile {
	return &FuseFile{node: f.node, name: f.name, bytesRead: f.bytesRead, allBytes: f.allBytes, filesRead: f.filesRead, allFiles: f.allFiles}
}

func Fuse2Dav(fs fs.FS) webdav.FileSystem {
	return webdav.FileSystem(&FuseDavFS{fs: fs})
}

func (fs *FuseDavFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return fmt.Errorf("Not implemented")
}
func (fs *FuseDavFS) RemoveAll(ctx context.Context, name string) error {
	return fmt.Errorf("Not implemented")
}
func (fs *FuseDavFS) Rename(ctx context.Context, oldName, newName string) error {
	return fmt.Errorf("Not implemented")
}

func (f *FuseDavFile) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("Not implemented")
}

func (fs *FuseDavFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	nfs := Fuse2NFS(fs.fs)
	return nfs.Stat(name)
}

func (f *FuseDavFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	node, err := findNode(ctx, f.fs, name)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(name, "/")
	return &FuseDavFile{node: node, name: parts[len(parts)-1]}, nil
}

func (f *FuseDavFile) Close() error {
	return nil
}

func (f *FuseDavFile) Read(p []byte) (int, error) {
	fusefile := &FuseFile{node: f.node, name: f.name}
	return fusefile.Read(p)
}

func (f *FuseDavFile) Readdir(count int) ([]os.FileInfo, error) {
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

func (f *FuseDavFile) Seek(offset int64, whence int) (int64, error) {
	nfs := f.ToNFSFile()
	nfs.Seek(offset, whence)
	f.bytesRead = nfs.bytesRead
	f.allBytes = nfs.allBytes
	return int64(f.bytesRead), nil
}

func (f *FuseDavFile) Stat() (os.FileInfo, error) {
	ctx := context.Background()
	attr := fuse.Attr{}
	err := f.node.Attr(ctx, &attr)
	if err != nil {
		return nil, err
	}
	return FuseAttr{attr: attr, name: f.name}, nil
}

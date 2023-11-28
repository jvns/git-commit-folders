package fuse2nfs

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	billy "github.com/go-git/go-billy/v5"
	nfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

type FuseNFSfs struct {
	fs fs.FS
}
type FuseAttr struct {
	attr fuse.Attr
	name string
}
type FuseFile struct {
	node      fs.Node
	name      string
	bytesRead int
	allBytes  []byte
	filesRead int
	allFiles  []os.FileInfo
}

func Fuse2NFS(fs fs.FS) billy.Filesystem {
	return &FuseNFSfs{fs: fs}
}

func RunServer(fs billy.Filesystem, port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	panicOnErr(err, "starting TCP listener")
	fmt.Printf("Server running at %s\n", listener.Addr())
	handler := nfshelper.NewNullAuthHandler(fs)
	cacheHelper := nfshelper.NewCachingHandler(handler, 1000)
	panicOnErr(nfs.Serve(listener, cacheHelper), "serving nfs")
}

func panicOnErr(err error, desc ...interface{}) {
	if err == nil {
		return
	}
	log.Println(desc...)
	log.Panicln(err)
}

/** THESE THINGS AREN'T IMPLEMENTED **/

func (fs *FuseNFSfs) Chroot(path string) (billy.Filesystem, error) {
	return nil, fmt.Errorf("not implemented")
}
func (fs *FuseNFSfs) Root() string {
	/* hope this doesn't cause a problem */
	return ""
}

func (fs *FuseNFSfs) Create(filename string) (billy.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *FuseNFSfs) MkdirAll(path string, perm os.FileMode) error {
	return fmt.Errorf("not implemented")
}
func (f *FuseFile) Lock() error {
	return fmt.Errorf("not implemented")
}

func (f *FuseFile) Unlock() error {
	return fmt.Errorf("not implemented")
}

func (f *FuseFile) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("not implemented")
}

func (f *FuseFile) Truncate(size int64) error {
	return fmt.Errorf("not implemented")
}

func (f *FuseNFSfs) Symlink(target, link string) error {
	return fmt.Errorf("not implemented")
}

func (f *FuseNFSfs) TempFile(dir, prefix string) (billy.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (fs *FuseNFSfs) Join(elem ...string) string {
	return strings.Join(elem, "/")
}

func (fs *FuseNFSfs) Remove(path string) error {
	return fmt.Errorf("not implemented")
}

func (fs *FuseNFSfs) Rename(from, to string) error {
	return fmt.Errorf("not implemented")
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
	// TODO: wrong
	return time.Unix(0, 0)
}

func (f FuseAttr) IsDir() bool {
	return f.attr.Mode.IsDir()
}

func (f FuseAttr) Sys() interface{} {
	return &syscall.Stat_t{
		Uid:   uint32(os.Getuid()),
		Gid:   uint32(os.Getgid()),
		Rdev:  0,
		Ino:   f.attr.Inode,
		Nlink: 1,
	}
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

func nodeToFileInfo(node fs.Node, filename string) (os.FileInfo, error) {
	ctx := context.Background()
	a := fuse.Attr{}
	err := node.Attr(ctx, &a)
	if err != nil {
		return nil, err
	}
	return FuseAttr{attr: a, name: filename}, nil
}

func (f *FuseNFSfs) Stat(path string) (os.FileInfo, error) {
	ctx := context.Background()
	node, err := findNode(ctx, f.fs, path)
	if err != nil {
		return nil, err
	}
	return nodeToFileInfo(node, getFilename(path))
}

func (f *FuseNFSfs) Lstat(filename string) (os.FileInfo, error) {
	return f.Stat(filename)
}

func getFilename(path string) string {
	parts := strings.Split(path, "/")
	/* return the last nonempty part */
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return "/"
}

func (f *FuseNFSfs) Open(path string) (billy.File, error) {
	ctx := context.Background()
	node, err := findNode(ctx, f.fs, path)
	if err != nil {
		return nil, err
	}
	return &FuseFile{node: node, name: getFilename(path)}, nil
}

func (f *FuseNFSfs) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	return f.Open(filename)
}

func (f *FuseNFSfs) ReadDir(path string) ([]os.FileInfo, error) {
	ctx := context.Background()
	node, err := findNode(ctx, f.fs, path)
	if err != nil {
		return nil, err
	}
	return getFileInfos(node)
}

func getFileInfos(node fs.Node) ([]os.FileInfo, error) {
	ctx := context.Background()
	if _, ok := node.(fs.HandleReadDirAller); !ok {
		return []os.FileInfo{}, nil
	}
	files, err := node.(fs.HandleReadDirAller).ReadDirAll(ctx)
	if err != nil {
		return nil, err
	}

	dirents := []os.FileInfo{}
	for _, file := range files {
		node, err := node.(fs.NodeStringLookuper).Lookup(ctx, file.Name)
		if err != nil {
			return nil, err
		}
		attr, err := nodeToFileInfo(node, file.Name)
		if err != nil {
			return nil, err
		}
		dirents = append(dirents, attr)
	}
	return dirents, nil
}

func (f *FuseNFSfs) Readlink(filename string) (string, error) {
	ctx := context.Background()
	node, err := findNode(ctx, f.fs, filename)
	if err != nil {
		return "", err
	}
	if n, ok := node.(fs.NodeReadlinker); ok {
		return n.Readlink(ctx, nil)
	}
	return "", fmt.Errorf("Node does not implement NodeReadlinker")
}

func (f *FuseFile) Close() error {
	return nil
}

func (f *FuseFile) Name() string {
	return f.name
}

func (f *FuseFile) ReadBytes() error {
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

func (f *FuseFile) Read(p []byte) (n int, err error) {
	err = f.ReadBytes()
	if err != nil {
		return 0, err
	}

	n = copy(p, f.allBytes[f.bytesRead:])
	f.bytesRead += n
	return n, nil
}

func (f *FuseFile) ReadAt(p []byte, off int64) (n int, err error) {
	err = f.ReadBytes()
	if err != nil {
		return 0, err
	}
	n = copy(p, f.allBytes[off:])
	return n, nil
}

func (f *FuseFile) Seek(offset int64, whence int) (int64, error) {
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

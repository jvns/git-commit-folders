package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/anacrolix/fuse"
	"github.com/anacrolix/fuse/fs"
	git "github.com/go-git/go-git/v5"
	myfuse "github.com/jvns/git-commit-folders/fuse"
	"github.com/jvns/git-commit-folders/fuse2nfs"
	"github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
	"golang.org/x/net/webdav"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:")
	fmt.Fprintf(os.Stderr, "  %s [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
}

type options struct {
	typ        string
	mountpoint string
	repoDir    string
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.typ, "type", "fuse", "type of mount (webdav, nfs, or fuse)")
	flag.StringVar(&opts.mountpoint, "mountpoint", "", "mountpoint")
	flag.StringVar(&opts.repoDir, "repo", ".", "repo dir")
	flag.Parse()
	if opts.mountpoint == "" {
		usage()
		log.Fatalf("Must specify mountpoint\n")
	}
	if opts.typ != "webdav" && opts.typ != "nfs" && opts.typ != "fuse" {
		usage()
		log.Fatalf("Invalid type %s\n", opts.typ)
	}
	return opts
}

func main() {
	opts := parseOptions()
	repo, err := git.PlainOpen(opts.repoDir)
	if err != nil {
		log.Fatal(err)
	}
	fs := myfuse.New(repo)

	createMountpoint(opts.mountpoint)
	if err != nil {
		log.Fatal(err)
	}
	if opts.typ == "webdav" {
		serveDav(fs, opts.mountpoint)
	} else if opts.typ == "nfs" {
		serveNFS(fs, opts.mountpoint)
	} else {
		serveFuse(fs, opts.mountpoint)
	}
}

func startListener() (net.Listener, int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Server running at", listener.Addr())
	port := listener.Addr().(*net.TCPAddr).Port
	return listener, port
}

func serveDav(fs fs.FS, mountpoint string) {
	davFS := fuse2nfs.Fuse2Dav(fs)
	srv := &webdav.Handler{
		FileSystem: davFS,
		LockSystem: webdav.NewMemLS(),
		Logger:     fuse2nfs.DebugLogger,
	}
	http.Handle("/", srv)

	listener, port := startListener()
	server := func() error {
		return http.Serve(listener, nil)
	}
	mountCmd := exec.Command("mount", "-t", "webdav", fmt.Sprintf("localhost:%d", port), mountpoint)
	serve(server, mountCmd, mountpoint)
}

func serveNFS(fs fs.FS, mountpoint string) {
	nfsFS := fuse2nfs.Fuse2NFS(fs)
	handler := nfshelper.NewNullAuthHandler(nfsFS)
	// was running into problems with stale file handles when this was set to
	// 1000 so I set this to a bigger number. I don't think that's the right
	// way to fix the problem but I don't know what is
	cacheHelper := nfshelper.NewCachingHandler(handler, 10000)
	listener, port := startListener()
	server := func() error {
		return nfs.Serve(listener, cacheHelper)
	}
	mountCmd := exec.Command("mount", "-o", fmt.Sprintf("port=%d,mountport=%d", port, port), "-t", "nfs", "localhost:/", mountpoint)
	serve(server, mountCmd, mountpoint)
}

func serveFuse(fuseFS fs.FS, mountpoint string) {
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("helloworld"),
		fuse.Subtype("hellofs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Hello world!"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	<-c.Ready

	server := func() error {
		defer c.Close()
		return fs.Serve(c, fuseFS)
	}
	serve(server, nil, mountpoint)
}

func serve(server func() error, mountCmd *exec.Cmd, mountpoint string) {
	serverDone := make(chan error)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT)
	signal.Notify(sigchan, syscall.SIGTERM)

	go func() {
		serverDone <- server()
		close(serverDone)
	}()
	time.Sleep(500 * time.Millisecond)
	if mountCmd != nil {
		if err := mountCmd.Run(); err != nil {
			log.Fatal(err)

		}
	}

	select {
	case <-sigchan:
		fmt.Println("Shutting down...")
	case err := <-serverDone:
		if err != nil {
			log.Fatal(err)
		}
	}
	umountCmd := exec.Command("umount", mountpoint)
	if err := umountCmd.Run(); err != nil {
		fmt.Printf("Error unmounting %s: %v\n", mountpoint, err)
	}
}

func createMountpoint(mountpoint string) (string, error) {
	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		os.Mkdir(mountpoint, 0755)
	}
	return mountpoint, nil
}

func panicOnErr(err error, desc ...interface{}) {
	if err == nil {
		return
	}
	log.Println(desc...)
	log.Panicln(err)
}

### git-commit-folders

Gives you a folder for every commit in your git repository. It's read only,
uses 0 disk space, and updates live based on the state of your git repository.

Extremely experimental software. 

### how to use it

```
go build
./git-commit-folders -type nfs
```

It'll mount a `.git/commit-folders` directory with all your commits in it

### how it works

It mounts a virtual filesystem (using NFS, fuse, or WebDav) and mounts it with
the `mount` command. It doesn't work on Windows but probably could be made to.

### NFS, FUSE, DAV

there are 3 different filesystem implementations. I'd suggest:

* `-type fuse` if you're on Linux
* `-type nfs` if you're on Mac OS (because FUSE on Mac is annoying)
* `-type webdav` for nobody, it's broken because I couldn't get symlinks on webdav to work. just leaving the webdav code in there in case it's salvageable.

You can try to use the FUSE version on Mac with MacFuse or FUSE-T if you want though.

### a tour of the folders

I might change all of this but right now there are four main subfolders.
`commits/` contains all the commits, and everything else is a symlink to a
commit.

```
$ ls .git/commit-folders
branches/  branch_histories/  commits/  tags/
```

**commits**

the `commits/` directory looks empty, but you can list any individual commit by its sha

```
$ ls .git/commit-folders/commits/da83dce00782814ecfd33ef6d968ff9e43188a94/
branches.go  commit.go  go.mod  go.sum  main.go  symlink.go
```


**tags**

```
$ ls .git/commit-folders/tags/
v0.000@
$ ls .git/commit-folders/tags/v0.000/
branches.go  branch_histories.go  commit.go  go.mod  go.sum  main.go  symlink.go  tags.go
```

**branches**

```
$ ls .git/commit-folders/branches/
main@  test@
$ ls .git/commit-folders/branches/main/
branches.go  branch_histories.go  commit.go  go.mod  go.sum  main.go  symlink.go
```

**branch histories**

shows the last 100 commits on a branch. They're numbered, 0 is the most recent.

here we'll look at the code from 4 versions ago

```
$ ls .git/commit-folders/branch_histories/main/
00-f1e4200744ae2fbe584d3ad3638cf61593a11624@  02-dc49186e766bcdb62a3958533a62d3fd626b253e@  04-b9c9e9f09cc918825066f105d62c550cc3c0958e@
01-03bf66122c3acf44fb781f27cd41415af75fcbe4@  03-da83dce00782814ecfd33ef6d968ff9e43188a94@  05-97d8dea79acb702b3ad66e08218c26c2fda9b1de@
$ ls .git/commit-folders/branch_histories/main/04-b9c9e9f09cc918825066f105d62c550cc3c0958e/
commit.go  go.mod  go.sum  main.go
```

### cool stuff you can do

you can go into your branch and grep for the code you deleted!

```
$ cd .git/commit-folders/branch_histories/main
$ grep 'func readBlob' */commit.go
03-fc450bb99460b9b793fcc36ca79b74caf6a9bc2a/commit.go:func readBlob(repo *git.Repository, id plumbing.Hash) ([]byte, error) {
```


### bugs

there are 1 million bugs and limitations. I may or may not fix any of them.

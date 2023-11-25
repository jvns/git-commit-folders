### git-commit-folders

gives you a folder for every commit in your git repository. It's read only.

### example usage

there are four sections. `commits/` contains a bunch of folders, and everything else is jus 

```
$ ls mnt
branches/  branch_histories/  commits/  tags/
```

**commits**

the `commits/` directory looks empty, but you can list any individual commit by its sha

```
$ ls mnt/commits/
$ ls mnt/commits/da83dce00782814ecfd33ef6d968ff9e43188a94/
branches.go  commit.go  go.mod  go.sum  main.go  symlink.go
```


**tags**

```
$ ls mnt/tags/
v0.000@
$ ls mnt/tags/v0.000/
branches.go  branch_histories.go  commit.go  go.mod  go.sum  main.go  symlink.go  tags.go
```

**branches**

```
$ ls mnt/branches/
main@  test@
$ ls mnt/branches/main/
branches.go  branch_histories.go  commit.go  go.mod  go.sum  main.go  symlink.go
```

**branch histories**

shows the last 20 commits on a branch

here we'll look at the code from 4 versions ago

```
$ ls mnt/branch_histories/main/
00-f1e4200744ae2fbe584d3ad3638cf61593a11624@  02-dc49186e766bcdb62a3958533a62d3fd626b253e@  04-b9c9e9f09cc918825066f105d62c550cc3c0958e@
01-03bf66122c3acf44fb781f27cd41415af75fcbe4@  03-da83dce00782814ecfd33ef6d968ff9e43188a94@  05-97d8dea79acb702b3ad66e08218c26c2fda9b1de@
$ ls mnt/branch_histories/main/04-b9c9e9f09cc918825066f105d62c550cc3c0958e/
commit.go  go.mod  go.sum  main.go
```

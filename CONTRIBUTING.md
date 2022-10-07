# Contributing to holster

## Formatting
Prior to commit all code should be formatted by the goimports tool.

### How to install
```shell
go install golang.org/x/tools/cmd/goimports@latest
```

### How to configure IDEs
#### GoLand
Installation of goimports is not needed.

Go to Preferences → Editor → Code style → Go → Imports → Sorting, choose “goimports“.

#### VSCode
1. Install Golang extension.
2. Go to Settings → Extensions → Format tool, choose "goimports".

#### Emacs, Vim, GoSublime
See [goimports doc](https://pkg.go.dev/golang.org/x/tools/cmd/goimports?tab=doc).

## Linters
We are using [golangci-lint](https://golangci-lint.run/) for static code analysis.

In case of `Lint / lint` GitHub Action failure check the annotations on the PR’s `Files changed` tab.

You could also run `make lint` on your local env to check it.

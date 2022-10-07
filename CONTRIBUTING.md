# Contributing to holster

## Go version
Use the same version as in the [go.mod](https://github.com/mailgun/holster/blob/master/go.mod#L3) file.

## go.mod and go.sum
`go mod tidy` should be called before commit to add missing and remove unused modules.

## Formatting
Prior to commit all code should be formatted by the [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) tool.

### How to configure IDEs
#### GoLand
Go to Preferences → Editor → Code style → Go → Imports → Sorting, choose "goimports".

#### VSCode
1. Install Golang extension.
2. Go to Settings → Extensions → Format tool, choose "goimports".

#### Emacs, Vim, GoSublime, etc.
See [goimports doc](https://pkg.go.dev/golang.org/x/tools/cmd/goimports?tab=doc).

## Linters
We are using [golangci-lint](https://golangci-lint.run/) for static code analysis.

In case of `Lint / lint` GitHub Action failure check the annotations on the PR’s `Files changed` tab.

You could also run `make lint` on your local env to check it.

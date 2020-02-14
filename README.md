# stx
Go-based CLI for managing CloudFormation stacks written in Cue

## Installation
1. Clone the repo to your local Go source folder. (It could be in your home folder: `~/go/src/github.com/TangoGroup/stx`)
2. From the folder with `main.go` run `go build ./*.go`
3. Soft link to the binary: `ln -s ~/go/src/github.com/TangoGroup/stx/main /usr/local/bin/stx`

## Usage

`stx [global flags] <command> [command flags] [./... or specific cue files]`

If no args are present after <command>, stx will default to using `./...` as a way to find Cue files. This can be overriden with specific files: `stx print ./text.cue`

### Commands
- `print` behaves like `cue export -out yml` but highlights errors, and the folders being evaluated
- `xpt` saves stacks to disk. See `config.stx.cue` for setting `Xpt: YmlPath:`
- `dpl` creates a changeset, previews changes, and prompts to execute

### Roadmap
- Add color to yaml output of `print`
- Add sts, sdf, exe, and events commands
- Add config options to use ykman for automatic mfa retrieval

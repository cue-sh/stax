# stax

## Commands

- add
- delete
- deploy
- diff
- events
- export
- import
- notify
- print
- resources
- save
- status

## Global Flags
- --environment, -e Includes only stacks with this environment.
- --profile Includes only stacks with this profile
- --region-code, -r  Includes only stacks with this region code
- --exclude Excludes subdirectory paths matching this regular expression.
- --include Includes subdirectory paths matching this regular expression.
- --stacks Includes only stacks whose name matches this regular expression.
- --has Includes only stacks that contain the provided path. E.g.: Template.Parameters
- --debug Enables verbose output of debug level messages.
- --no-color Disables color output. Useful for reducing noise on systems that don't support color codes.

## Arguments

Stx accepts any number non-flagged arguments: that is the path to the files that should be evaluated by Cue. The default, when no argument is provided, is `./...` which is equivalent to `cue export ./...` This will evaluate any `.cue` files in the current directory, along with any in parent and all sub-directories. This is known as `instances`. Each instance will produce a list of `Stacks` and each stack in that list will be processed according to the provided sub-command.

Stx can process any cue files that cue can process, so long as they conform to the Stacks pattern. If you need to process files that don't leverage instances, you can specify files individually, or through file globbing.
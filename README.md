# Todaily

Todaily is a simple and minimal daily habit tracker built with
[Gio](https://gioui.org/). You start by creating a list of things you want to
do every day. That list will then be presented to you every day going forward.

## Development

To build the app, run `go build` (or just `go build -tags nowayland` for no Wayland
support).

If you have [`task`](https://github.com/go-task/task),
[`goimports`](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) and
[`gofumpt`](https://github.com/mvdan/gofumpt) installed, you can simply run
`task` (or `task nowayland`) to fmt, lint and build the project.

## License

This is free and unencumbered software released into the public domain. Please
see the [UNLICENSE](./UNLICENSE) file for more information.

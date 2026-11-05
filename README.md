# jaira plugin template

A working [jaira](https://github.com/BeMuCa/jaira) plugin to start yours from:
a Go program `jaira-hello`, its manifest `jaira-plugin.json`, tests, and a
release pipeline that builds exactly what the jaira plugin catalogue accepts.

The standard every plugin follows is `docs/PLUGINS.md` in the jaira
repository: how jaira starts a plugin, the environment it gets, the exit
codes, the manifest, and why a plugin writes only through the `jaira` CLI.

## From template to catalogue in three steps

### 1. Make it yours

Press **Use this template** (do not fork), clone your new repository, then
edit `jaira-plugin.json`:

- `id` — lower-case letters, digits and dashes, without `jaira` in it. The
  program becomes `jaira-<id>`; nothing else in the repository names the id.
- `name`, `commands`, `capabilities` — what a person reads before installing.
- `version` — `0.1.0` to start; `minJairaVersion` — the oldest jaira you
  tested with.

Replace `greet` in `main.go` with your own commands. Keep `manifest --json`:
jaira uses it to learn what an installed plugin offers.

```bash
go test ./...      # also checks jaira-plugin.json against the standard
go build -o jaira-$(jq -r .id jaira-plugin.json) .
```

### 2. Release it

```bash
git tag v0.1.0 && git push --tags
```

The `release` workflow checks that the tag is `v` + `version` from
`jaira-plugin.json`, runs the tests, and publishes with goreleaser:

- `jaira-<id>_<os>_<arch>` for `linux`, `darwin`, `windows` (`.exe`) ×
  `amd64`, `arm64`
- `checksums.txt`
- `jaira-plugin.json`

Every later release is the same two steps: raise `version`, tag `v<version>`.
An update needs no new pull request to the catalogue.

### 3. Get it listed

1. Fork <https://github.com/panda-bobr/jaira-releases>.
2. Append your entry to `plugins.json`:

   ```json
   {
     "id": "hello",
     "name": "Hello",
     "author": "Your Name",
     "description": "One sentence on what it does.",
     "repo": "https://github.com/you/jaira-hello"
   }
   ```

3. Open a pull request. Its CI checks the entry, that the id is free, that
   your repository has a release with every asset and `checksums.txt`, and
   that you own the repository. A maintainer merges it.

## The example: `jaira-hello`

```bash
jaira hello greet            # "Hello! This board has 12 tickets."
jaira hello greet A3K9QP     # leaves a note on A3K9QP through 'jaira note'
jaira-hello manifest --json  # prints jaira-plugin.json
```

It reads the board with `jaira list --json` (on `$JAIRA_BOARD` when set),
writes with `jaira note`, refuses with exit 3 when `$JAIRA_VERSION` is older
than `minJairaVersion`, and passes jaira's own exit code on when a call fails.

Until jaira can start installed plugins itself (`jaira <id> ...`), run the
binary directly from inside a repository with a jaira board:
`./jaira-hello greet`.

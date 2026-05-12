# Distribution

Use this reference for install, update, GitHub Releases, and release diagnostics.

## Installer

One-line install:

```bash
curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

Specific version:

```bash
WHOOP_CLI_VERSION=v0.1.1 curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

Custom install directory:

```bash
WHOOP_CLI_INSTALL_DIR="$HOME/bin" curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

The installer expects:

- a GitHub Release
- platform archives for OS/arch
- `checksums.txt`

## Self Update

Check:

```bash
whoop-cli update --check --json
```

Update:

```bash
whoop-cli update --json
```

Specific version:

```bash
whoop-cli update --version v0.1.1 --json
```

Test safely without overwriting a real binary:

```bash
whoop-cli update --install-dir "$(mktemp -d)" --json
```

## Release Workflow

Release publishing is tag-driven:

```bash
git tag v0.1.1
git push origin v0.1.1
```

The GitHub Action:

- runs `go test ./...`
- builds macOS, Linux, and Windows binaries
- injects the tag into `main.version`
- packages archives
- generates `checksums.txt`
- publishes a GitHub Release

Expected asset names:

```text
whoop-cli_v0.1.1_darwin_amd64.tar.gz
whoop-cli_v0.1.1_darwin_arm64.tar.gz
whoop-cli_v0.1.1_linux_amd64.tar.gz
whoop-cli_v0.1.1_linux_arm64.tar.gz
whoop-cli_v0.1.1_windows_amd64.zip
checksums.txt
```

## Release Diagnostics

If install/update fails:

1. Check that the tag workflow completed.
2. Check that the GitHub Release exists.
3. Check that the current OS/arch has an asset.
4. Check that `checksums.txt` includes the exact asset filename.
5. Check install directory permissions.

Commands:

```bash
whoop-cli update --check --json
whoop-cli version
```

If debugging from source, use local tests:

```bash
go test ./...
go vet ./...
sh -n install.sh
```

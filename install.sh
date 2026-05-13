#!/bin/sh
set -eu

repo="${WHOOP_CLI_REPO:-MohitTilwani15/whoop-cli}"
version="${WHOOP_CLI_VERSION:-latest}"
install_dir="${WHOOP_CLI_INSTALL_DIR:-$HOME/.local/bin}"
bin_name="whoop-cli"

say() {
	printf '%s\n' "$*"
}

fail() {
	printf 'whoop-cli install: %s\n' "$*" >&2
	exit 1
}

have() {
	command -v "$1" >/dev/null 2>&1
}

download() {
	url="$1"
	out="$2"
	if have curl; then
		curl -fsSL "$url" -o "$out"
	elif have wget; then
		wget -qO "$out" "$url"
	else
		fail "curl or wget is required"
	fi
}

detect_os() {
	case "$(uname -s)" in
	Darwin) printf 'darwin' ;;
	Linux) printf 'linux' ;;
	CYGWIN* | MINGW* | MSYS*) printf 'windows' ;;
	*) fail "unsupported OS: $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) printf 'amd64' ;;
	arm64 | aarch64) printf 'arm64' ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
	esac
}

sha256_file() {
	file="$1"
	if have sha256sum; then
		sha256sum "$file" | awk '{print $1}'
	elif have shasum; then
		shasum -a 256 "$file" | awk '{print $1}'
	else
		fail "sha256sum or shasum is required"
	fi
}

checksum_for_asset() {
	checksums="$1"
	asset="$2"
	awk -v asset="$asset" '
		$0 ~ asset {
			for (i = 1; i <= NF; i++) {
				if ($i ~ /^[0-9a-fA-F]{64}$/) {
					print tolower($i)
					exit
				}
			}
		}
	' "$checksums"
}

tmp="${TMPDIR:-/tmp}/whoop-cli-install.$$"
cleanup() {
	rm -rf "$tmp"
}
trap cleanup EXIT INT TERM
mkdir -p "$tmp/extract"

os="$(detect_os)"
arch="$(detect_arch)"

if [ "$os" = "windows" ]; then
	binary_name="$bin_name.exe"
else
	binary_name="$bin_name"
fi

if [ -n "${WHOOP_CLI_RELEASE_API:-}" ]; then
	release_api="$WHOOP_CLI_RELEASE_API"
elif [ "$version" = "latest" ]; then
	release_api="https://api.github.com/repos/$repo/releases/latest"
else
	release_api="https://api.github.com/repos/$repo/releases/tags/$version"
fi

release_json="$tmp/release.json"
download "$release_api" "$release_json"

tag="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$release_json" | head -n 1)"
[ -n "$tag" ] || fail "could not resolve release tag for $repo"

asset_url="$(
	sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$release_json" |
		grep -Ei "$os.*$arch.*(tar\.gz|tgz|zip)$" |
		grep -Evi '(checksums?|sha256)' |
		head -n 1
)"

if [ -z "$asset_url" ]; then
	say "Available release assets:"
	sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/  - \1/p' "$release_json" >&2
	fail "no release asset found for $os/$arch in $repo@$tag"
fi

checksums_url="$(
	sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$release_json" |
		grep -Ei '(checksums?|sha256).*\.txt$' |
		head -n 1
)"

[ -n "$checksums_url" ] || fail "release $tag does not include checksums.txt"

asset_name="$(basename "$asset_url")"
archive="$tmp/$asset_name"
checksums="$tmp/checksums.txt"

say "Installing whoop-cli $tag for $os/$arch"
download "$asset_url" "$archive"
download "$checksums_url" "$checksums"

expected="$(checksum_for_asset "$checksums" "$asset_name")"
[ -n "$expected" ] || fail "checksums.txt does not include $asset_name"

actual="$(sha256_file "$archive")"
[ "$actual" = "$expected" ] || fail "checksum mismatch for $asset_name"

case "$asset_name" in
*.tar.gz | *.tgz)
	tar -xzf "$archive" -C "$tmp/extract"
	;;
*.zip)
	have unzip || fail "unzip is required for zip archives"
	unzip -q "$archive" -d "$tmp/extract"
	;;
*)
	fail "unsupported archive format: $asset_name"
	;;
esac

binary_path="$(find "$tmp/extract" -type f -name "$binary_name" | head -n 1)"
[ -n "$binary_path" ] || fail "archive did not contain $binary_name"

mkdir -p "$install_dir"
cp "$binary_path" "$install_dir/$binary_name"
chmod 0755 "$install_dir/$binary_name"

say "Installed $install_dir/$binary_name"

if "$install_dir/$binary_name" version >/dev/null 2>&1; then
	"$install_dir/$binary_name" version
fi

case ":$PATH:" in
*":$install_dir:"*) ;;
*)
	say ""
	say "Add $install_dir to PATH to run whoop-cli from anywhere."
	say "For zsh/bash:"
	say "  export PATH=\"$install_dir:\$PATH\""
	;;
esac

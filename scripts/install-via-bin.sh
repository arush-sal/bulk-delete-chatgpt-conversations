#!/bin/sh

set -eu

BIN_RELEASES_LATEST_URL="https://github.com/marcosnils/bin/releases/latest"
TOOL_REPOSITORY="github.com/arush-sal/bulk-delete-chatgpt-conversations"
TOOL_RELEASES_LATEST_URL="https://github.com/arush-sal/bulk-delete-chatgpt-conversations/releases/latest"

log() {
	printf '%s\n' "$*" >&2
}

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

command_exists() {
	command -v "$1" >/dev/null 2>&1
}

download_to() {
	url=$1
	destination=$2

	if command_exists curl; then
		curl -fsSL "$url" -o "$destination"
		return 0
	fi
	if command_exists wget; then
		wget -qO "$destination" "$url"
		return 0
	fi

	fail "curl or wget is required"
}

resolve_final_url() {
	url=$1

	if command_exists curl; then
		curl -fsSL -o /dev/null -w '%{url_effective}' "$url"
		return 0
	fi
	if command_exists wget; then
		wget -S --max-redirect=20 -O /dev/null "$url" 2>&1 | awk '
			tolower($1) == "location:" {
				location = $2
			}
			END {
				gsub(/\r/, "", location)
				print location
			}
		'
		return 0
	fi

	fail "curl or wget is required"
}

detect_os() {
	case "$(uname -s)" in
		Darwin)
			printf 'darwin\n'
			;;
		Linux)
			printf 'linux\n'
			;;
		*)
			fail "unsupported operating system: $(uname -s)"
			;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64|amd64)
			printf 'amd64\n'
			;;
		arm64|aarch64)
			printf 'arm64\n'
			;;
		*)
			fail "unsupported architecture: $(uname -m)"
			;;
	esac
}

extract_release_tag() {
	final_url=$1
	printf '%s\n' "${final_url##*/}"
}

print_path_hint() {
	install_dir=$1

	case ":$PATH:" in
		*:"$install_dir":*)
			return 0
			;;
	esac

	log ""
	log "Add $install_dir to your PATH if needed:"
	log "  export PATH=\"$install_dir:\$PATH\""
}

bootstrap_bin() {
	install_dir=$1
	os_name=$(detect_os)
	arch_name=$(detect_arch)

	tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t chatgpt-bulk-bin)
	trap 'rm -rf "$tmpdir"' EXIT INT TERM HUP

	release_tag=$(extract_release_tag "$(resolve_final_url "$BIN_RELEASES_LATEST_URL")")
	[ -n "$release_tag" ] || fail "could not resolve the latest marcosnils/bin release"

	release_version=${release_tag#v}
	asset_url="https://github.com/marcosnils/bin/releases/download/${release_tag}/bin_${release_version}_${os_name}_${arch_name}"
	asset_path="$tmpdir/bin"
	download_to "$asset_url" "$asset_path"

	mkdir -p "$install_dir"
	chmod +x "$asset_path"
	bin_path="$install_dir/bin"
	mv "$asset_path" "$bin_path"
	printf '%s\n' "$bin_path"
}

main() {
	if [ -z "${HOME:-}" ]; then
		fail "HOME must be set"
	fi
	install_dir=${BIN_INSTALL_DIR:-"$HOME/.local/bin"}
	tool_path="$install_dir/chatgpt-bulk"

	if command_exists bin; then
		bin_command=$(command -v bin)
		log "Using existing bin at $bin_command"
	else
		log "Installing bin into $install_dir"
		bin_command=$(bootstrap_bin "$install_dir")
		log "Installed bin at $bin_command"
	fi

	tool_release_tag=$(extract_release_tag "$(resolve_final_url "$TOOL_RELEASES_LATEST_URL")")
	[ -n "$tool_release_tag" ] || fail "could not resolve the latest chatgpt-bulk release"

	log "Installing chatgpt-bulk from GitHub releases"
	mkdir -p "$install_dir"
	PATH="$install_dir:$PATH" "$bin_command" install "${TOOL_REPOSITORY}/releases/tag/${tool_release_tag}" "$tool_path"

	log ""
	log "Installed chatgpt-bulk. Verify with:"
	log "  $tool_path --version"
	print_path_hint "$install_dir"
}

main "$@"

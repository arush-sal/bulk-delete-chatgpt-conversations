#!/bin/sh

set -eu

BIN_RELEASES_API_URL="https://api.github.com/repos/marcosnils/bin/releases"
TOOL_REPOSITORY="github.com/arush-sal/bulk-delete-chatgpt-conversations"

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

fetch_to_stdout() {
	url=$1

	if command_exists curl; then
		if [ -n "${GITHUB_AUTH_TOKEN:-}" ]; then
			curl -fsSL \
				-H "Accept: application/vnd.github+json" \
				-H "Authorization: Bearer ${GITHUB_AUTH_TOKEN}" \
				"$url"
			return 0
		fi
		curl -fsSL "$url"
		return 0
	fi
	if command_exists wget; then
		if [ -n "${GITHUB_AUTH_TOKEN:-}" ]; then
			wget -qO - \
				--header="Accept: application/vnd.github+json" \
				--header="Authorization: Bearer ${GITHUB_AUTH_TOKEN}" \
				"$url"
			return 0
		fi
		wget -qO - "$url"
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

find_bin_asset_url() {
	os_name=$1
	arch_name=$2
	asset_suffix="_${os_name}_${arch_name}"

	if command_exists jq; then
		fetch_to_stdout "$BIN_RELEASES_API_URL" |
			jq -r '.[0].assets[].browser_download_url' |
			grep "${asset_suffix}\$" |
			head -n 1
		return 0
	fi

	fetch_to_stdout "$BIN_RELEASES_API_URL" |
		sed -n "s/.*\"browser_download_url\": \"\\(https:[^\"]*${asset_suffix}\\)\".*/\\1/p" |
		head -n 1
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

download_and_install() {
	name=$1
	url=$2
	install_dir=$3

	[ -n "$url" ] || fail "could not resolve a download URL for ${name}"

	tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t chatgpt-bulk-bin)
	trap 'rm -rf "$tmpdir"' EXIT INT TERM HUP

	asset_path="$tmpdir/$name"
	download_to "$url" "$asset_path"

	mkdir -p "$install_dir"
	chmod +x "$asset_path"
	mv "$asset_path" "$install_dir/$name"
}

bootstrap_bin() {
	install_dir=$1
	os_name=$(detect_os)
	arch_name=$(detect_arch)
	asset_url=$(find_bin_asset_url "$os_name" "$arch_name")
	download_and_install "bin" "$asset_url" "$install_dir"
}

main() {
	if [ -z "${HOME:-}" ]; then
		fail "HOME must be set"
	fi
	install_dir=${BIN_INSTALL_DIR:-"$HOME/.local/bin"}
	tool_path="$install_dir/chatgpt-bulk"
	path_with_install_dir="$install_dir:$PATH"

	if hash bin 2>/dev/null; then
		bin_command=$(command -v bin)
		log "Using existing bin at $bin_command"
	else
		log ""
		log "Downloading bin..."
		bootstrap_bin "$install_dir"
		hash -r 2>/dev/null || true
		bin_command="$install_dir/bin"
		log "Installed bin at $bin_command"
	fi

	log "Installing chatgpt-bulk from GitHub releases"
	mkdir -p "$install_dir"
	PATH="$path_with_install_dir" "$bin_command" install "${TOOL_REPOSITORY}" "$tool_path"

	log ""
	log "Installed chatgpt-bulk. Verify with:"
	log "  $tool_path --version"
	print_path_hint "$install_dir"
}

main "$@"

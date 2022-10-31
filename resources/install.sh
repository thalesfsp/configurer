#!/usr/bin/env sh

######
# This script downloads the latest release from GitHub and installs it at 
# /usr/local/bin.
#
# Author: Thales Pinheiro
######

######
# Helpers.
######

ORG_NAME="thalesfsp"
APP_NAME="configurer"
BIN_DIR="/usr/local/bin"

# Replace BIN_DIR with args if provided.
if [ $# -gt 0 ]; then
  BIN_DIR=$1
fi

BOLD="$(tput bold 2>/dev/null || printf '')"
GREY="$(tput setaf 0 2>/dev/null || printf '')"
UNDERLINE="$(tput smul 2>/dev/null || printf '')"
RED="$(tput setaf 1 2>/dev/null || printf '')"
GREEN="$(tput setaf 2 2>/dev/null || printf '')"
YELLOW="$(tput setaf 3 2>/dev/null || printf '')"
BLUE="$(tput setaf 4 2>/dev/null || printf '')"
MAGENTA="$(tput setaf 5 2>/dev/null || printf '')"
NO_COLOR="$(tput sgr0 2>/dev/null || printf '')"

info() {
  printf '%s\n' "${BOLD}${GREY}>${NO_COLOR} $*"
}

warn() {
  printf '%s\n' "${YELLOW}! $*${NO_COLOR}"
}

error() {
  printf '%s\n' "${RED}x $*${NO_COLOR}" >&2
}

completed() {
  printf '%s\n' "${GREEN}✓${NO_COLOR} $*"
}

has() {
  command -v "$1" 1>/dev/null 2>&1
}

confirm() {
  if [ -z "${FORCE-}" ]; then
    printf "%s " "${MAGENTA}?${NO_COLOR} $* ${BOLD}[y/N]${NO_COLOR}"
    set +e
    read -r yn </dev/tty
    rc=$?
    set -e
    if [ $rc -ne 0 ]; then
      error "Error reading from prompt (please re-run with the '--yes' option)"
      exit 1
    fi
    if [ "$yn" != "y" ] && [ "$yn" != "yes" ]; then
      error 'Aborting (please answer "yes" to continue)'
      exit 1
    fi
  fi
}

# Get the latest release version from GitHub.
version=$(curl -s https://api.github.com/repos/${ORG_NAME}/${APP_NAME}/releases/latest | grep tag_name | cut -d '"' -f 4)

# Detect the architecture.
arch=$(uname -m)
case $arch in
  x86_64)
    arch="amd64"
    ;;
  arm64)
    arch="arm64"
    ;;
  *)
    error "Unsupported architecture: $arch"
    exit 1
    ;;
esac

# Detect the OS.
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux*)
    os="linux"
    ;;
  darwin*)
    os="darwin"
    ;;
  *)
    error "Unsupported operating system: $os"
    exit 1
    ;;
esac

# Detect if it has curl or wget and store in a variable called fetcher.
fetcher() {
    if has "curl"; then
        # Use curl save to directory.
        printf "%s" "curl -L --fail --silent --show-error -o"
    elif has "wget"; then
        printf "%s" "wget --quiet --output-document"
    else
        error "curl or wget is required"
        exit 1
    fi
}

# Remove "v" from the version string.
versionWithoutV=${version#v}

# Parse URL.
final_url=$(printf "https://github.com/%s/%s/releases/download/%s/%s_%s_%s_%s.tar.gz" "$ORG_NAME" "$APP_NAME" "$version" "$APP_NAME" "$versionWithoutV" "$os" "$arch")

######
# Starts here.
######

# Create a temp directory.
tmp_dir=$(mktemp -d)

info "Architecture: ${UNDERLINE}${BLUE}$arch${NO_COLOR}"
info "OS: ${UNDERLINE}${BLUE}$os${NO_COLOR}"
info "Temporary Filepath: ${UNDERLINE}${BLUE}$tmp_dir/$APP_NAME.tar.gz${NO_COLOR}"
info "Tarball URL: ${UNDERLINE}${BLUE}${final_url}${NO_COLOR}"

# Don't ask for confirmation if non-interactive.
if [ -t 0 ]; then
  confirm "Install $APP_NAME ${GREEN}latest ($version)${NO_COLOR} version to ${BOLD}${GREEN}${BIN_DIR}${NO_COLOR}?"
fi

# Download the latest release using fetcher
info "Downloading $final_url"
eval "$(fetcher)" "$tmp_dir/$APP_NAME.tar.gz" "$final_url"

# Unpack the archive in a temp directory.
info "Unpacking archive"
tar -xzf "$tmp_dir/$APP_NAME.tar.gz" -C "$tmp_dir"

# Move the binary to BIN_DIR, use sudo only if necessary.
if [ -w "$BIN_DIR" ]; then
  info "Moving binary to $BIN_DIR"
  mv "$tmp_dir/$APP_NAME" "$BIN_DIR"
else
  info "Moving binary to $BIN_DIR using sudo"
  sudo mv "$tmp_dir/$APP_NAME" "$BIN_DIR"
fi

# Notify the user.
info "$APP_NAME installed successfully"

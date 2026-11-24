#!/usr/bin/env bash
# Save a portrait, voice sample or seed into the avatar's folder, and open
# files from it in the creator's own viewer, so they can look and listen
# before choosing in chat.
#
#   media.sh save <https-url> <avatar-folder> <faces|voices|seeds> <name>
#   media.sh show <file>...
#   media.sh open <file>...
#
# save writes <avatar-folder>/media/<kind>/<name>.<ext> and prints that path.
# The extension comes from the link's path, else from the server's content
# type; only image, audio and mp4 types are kept, up to 60 MB. Server links
# expire (clips after 30 days, portraits and voices after 7, their links sooner, seed links in
# about an hour),
# so save before showing anything.
#
# show prints a Markdown block per file, separated by blank lines, for the
# agent to put in its reply, chosen by where the agent runs (detect-host,
# next to this script). Every block has a file:// link the creator can
# click to open the file. In Claude Desktop's Code tab, an image's block also
# shows it inline from the local file, and the first voice sample or seed
# is opened (not played) in the system player, since the chat has none; the
# rest are links the creator clicks.
#
# open takes portraits, voice samples and seeds (the same types save keeps)
# inside the project's atmee-avatars/ folder, as save prints them, and opens
# each with the system viewer. It fails, naming the files,
# when there is no viewer (no display, a remote shell): give the creator the
# paths instead.
#
# Like upload-file.sh: https only, regular files inside the project only, no
# symlinks, hidden files or hidden folders, and deliberately no overrides,
# because an instruction hidden in a document could set one on the command line.
set -euo pipefail

die() { printf 'media.sh: %s\n' "$1" >&2; exit 1; }
usage="usage: media.sh save <https-url> <avatar-folder> <faces|voices|seeds> <name> | media.sh show <file>... | media.sh open <file>..."
max_bytes=$((60 * 1024 * 1024))

no_hidden() {
  local part
  local -a parts
  IFS=/ read -r -a parts <<< "$1"
  for part in "${parts[@]}"; do
    case "$part" in
      .|..) ;;
      .*) die "refusing a hidden file or anything in a hidden folder: $1" ;;
    esac
  done
}

inside_avatars() {
  local p="$1" real
  if [ -d "$p" ]; then real="$(cd "$p" && pwd -P)"
  else real="$(cd "$(dirname "$p")" && pwd -P)/$(basename "$p")"; fi
  case "$real" in
    "$(pwd -P)"/atmee-avatars/*) printf '%s' "$real" ;;
    *) die "not inside this project's atmee-avatars/ folder: $p" ;;
  esac
}

ext_from_name() {
  local e
  e="$(printf '%s' "${1##*.}" | tr '[:upper:]' '[:lower:]')"
  case "$e" in
    jpeg) echo jpg ;;
    jpg|png|webp|mp3|wav|m4a|aac|ogg|opus|webm|flac|mp4) echo "$e" ;;
  esac
}

ext_from_type() {
  case "$(printf '%s' "${1%%;*}" | tr '[:upper:]' '[:lower:]' | tr -d ' ')" in
    image/jpeg) echo jpg ;;
    image/png) echo png ;;
    image/webp) echo webp ;;
    audio/mpeg|audio/mp3) echo mp3 ;;
    audio/wav|audio/x-wav|audio/wave) echo wav ;;
    audio/mp4|audio/x-m4a) echo m4a ;;
    audio/aac) echo aac ;;
    audio/ogg) echo ogg ;;
    audio/opus) echo opus ;;
    audio/webm|video/webm) echo webm ;;
    audio/flac|audio/x-flac) echo flac ;;
    video/mp4) echo mp4 ;;
  esac
}

save() {
  [ $# -eq 4 ] || die "$usage"
  local url="$1" folder="$2" kind="$3" name="$4"
  case "$url" in https://*) ;; *) die "refusing a link that is not https: $url" ;; esac
  case "$kind" in faces|voices|seeds) ;; *) die "kind must be faces, voices or seeds: $kind" ;; esac
  printf '%s' "$name" | grep -Eq '^[A-Za-z0-9_-]{1,80}$' || die "not a usable file name: $name"
  no_hidden "$folder"
  [ ! -L "$folder" ] || die "refusing a symlink: $folder"
  [ -d "$folder" ] || die "no such avatar folder: $folder"
  local dir sub
  dir="$(inside_avatars "$folder")"
  for sub in media "media/$kind"; do
    [ ! -L "$dir/$sub" ] || die "refusing a symlink: $folder/$sub"
  done
  dir="$dir/media/$kind"
  mkdir -p "$dir"

  local path="${url%%[?#]*}" tmp type ext size
  tmp="$(mktemp "$dir/.download.XXXXXX")"
  trap 'rm -f "$tmp"' EXIT
  type="$(curl -sS -fL --proto '=https' --proto-redir '=https' --max-time 120 --max-filesize "$max_bytes" -o "$tmp" -w '%{content_type}' "$url" 2>&1)" \
    || die "could not download $url: $type"
  size="$(wc -c < "$tmp" | tr -d ' ')"
  [ "$size" -le "$max_bytes" ] || die "file is over 60 MB: $url"
  ext="$(ext_from_name "${path##*/}")"
  [ -n "$ext" ] || ext="$(ext_from_type "$type")"
  [ -n "$ext" ] || die "unsupported file type (${type:-unknown}); expected jpg, png, webp, mp3, wav, m4a, aac, ogg, opus, webm, flac or mp4"
  mv "$tmp" "$dir/$name.$ext"
  trap - EXIT
  printf '%s\n' "${folder%/}/media/$kind/$name.$ext"
}

opener() {
  case "$(uname -s)" in
    Darwin) command -v open >/dev/null 2>&1 && echo open ;;
    MINGW*|MSYS*|CYGWIN*) command -v cmd.exe >/dev/null 2>&1 && echo cmd.exe ;;
    *) command -v xdg-open >/dev/null 2>&1 && echo xdg-open ;;
  esac
}

# Opens one file with the opener. Windows needs `start` and a Windows path,
# and Git Bash and MSYS2 must not rewrite `/c` into a drive path.
launch() {
  local tool="$1" real="$2"
  if [ "$tool" = cmd.exe ]; then
    command -v cygpath >/dev/null 2>&1 && real="$(cygpath -w "$real")"
    MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' cmd.exe /c start "" "$real"
  else
    "$tool" "$real"
  fi
}

# The checked absolute paths of the files given, in order, into reals.
check_files() {
  [ $# -ge 1 ] || die "$usage"
  local f
  reals=()
  for f in "$@"; do
    no_hidden "$f"
    [ ! -L "$f" ] || die "refusing a symlink; use the file itself: $f"
    [ -f "$f" ] || die "no such file: $f"
    [ -n "$(ext_from_name "$(basename "$f")")" ] \
      || die "only portraits, voice samples and seeds can be opened (jpg, png, webp, mp3, wav, m4a, aac, ogg, opus, webm, flac, mp4): $f"
    reals+=("$(inside_avatars "$f")")
  done
}

# A path as a file:// URL, with every byte but unreserved characters and /
# percent-encoded (the C locale walks bytes, so UTF-8 names encode right).
file_url() {
  local LC_ALL=C s="$1" out="" c i
  for ((i = 0; i < ${#s}; i++)); do
    c="${s:i:1}"
    case "$c" in
      [A-Za-z0-9/._~-]) out+="$c" ;;
      *) out+="$(printf '%%%02X' "$(( $(printf '%d' "'$c") & 255 ))")" ;;
    esac
  done
  printf 'file://%s' "$out"
}

# A path for an inline image in Claude Desktop, which shows a bare absolute or
# relative path but not one in <...>, a file:// URL or a percent-encoded one.
# A path that would end the markdown link (a space in the project folder, say)
# falls back to the path from the project folder (check_files keeps every file
# inside it), which Desktop resolves against the session's folder; a file name
# that would still end the link gets no inline image.
image_path() {
  local p="$1"
  case "$p" in *[[:space:]\(\)\<\>\[\]]*) ;; *) printf '%s' "$p"; return ;; esac
  p="${p#"$(pwd -P)"/}"
  case "$p" in *[[:space:]\(\)\<\>\[\]]*) return 1 ;; esac
  printf '%s' "$p"
}

show_files() {
  local -a reals
  check_files "$@"
  local desktop=0 real base img tool="" opened=0
  [ "$("$BASH" "${BASH_SOURCE[0]%/*}/detect-host")" = "claude-code desktop" ] && desktop=1
  [ $desktop -eq 1 ] && { tool="$(opener)" || true; }
  local first=1
  for real in "${reals[@]}"; do
    base="$(basename "$real")"
    [ $first -eq 1 ] || printf '\n'
    first=0
    # Only the first voice sample or seed opens, so one player window shows;
    # the player may not come forward, so the line says to click the link then.
    case "$desktop:$opened:$(ext_from_name "$base")" in
      1:*:jpg|1:*:png|1:*:webp)
        img="$(image_path "$real")" && printf '![%s](%s)\n' "$base" "$img"
        ;;
      1:0:*)
        opened=1
        [ -n "$tool" ] && launch "$tool" "$real" >/dev/null 2>&1 \
          && printf '%s is opening in your player; if nothing appears, click its link.\n' "$base"
        ;;
    esac
    printf '[Open %s](%s)\n' "$base" "$(file_url "$real")"
  done
}

open_files() {
  local real
  local -a reals
  check_files "$@"
  local tool
  tool="$(opener)" || true
  [ -n "$tool" ] || die "no viewer to open files with here; give the creator these paths: $*"
  local opened=0 i
  local -a failed=()
  for i in "${!reals[@]}"; do
    real="${reals[$i]}"
    launch "$tool" "$real" >/dev/null 2>&1 && opened=$((opened + 1)) || failed+=("${@:i+1:1}")
  done
  printf 'opened %s of %s\n' "$opened" "$#"
  [ ${#failed[@]} -eq 0 ] || die "could not open: ${failed[*]}"
}

[ $# -ge 1 ] || die "$usage"
cmd="$1"; shift
case "$cmd" in
  save) save "$@" ;;
  show) show_files "$@" ;;
  open) open_files "$@" ;;
  *) die "$usage" ;;
esac

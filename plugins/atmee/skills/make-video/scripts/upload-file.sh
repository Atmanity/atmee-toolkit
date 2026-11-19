#!/usr/bin/env bash
# Upload one of the creator's files to an Atmee upload link.
#
#   upload-file.sh <upload-url> <file>
#
# The upload URL is the uploadUrl from the avatars server's create_upload.
# The Content-Type comes from the file's extension, never from an argument:
# declare the same type to create_upload (it accepts only the declared one).
# Only signed-upload links on Atmee's own Supabase projects are accepted, and
# only regular files inside the current project: no symlinks, hidden files or
# hidden folders. There are deliberately no overrides, because an instruction
# hidden in a document the agent reads could set one on the same command line.
set -euo pipefail

die() { printf 'upload-file.sh: %s\n' "$1" >&2; exit 1; }

[ $# -eq 2 ] || die "usage: upload-file.sh <upload-url> <file>"
url="$1"
file="$2"

case "$url" in
  https://*) ;;
  *) die "not an upload link from atmee.ai (must be https): $url" ;;
esac
authority="${url#https://}"
authority="${authority%%[/?#]*}"
# Only a plain host name and an optional port: anything else (user info, a
# backslash, a fragment) can make curl connect somewhere other than it looks.
if ! printf '%s' "$authority" | grep -Eq '^[A-Za-z0-9.-]+(:[0-9]+)?$'; then
  die "not an upload link from atmee.ai: $url"
fi
host="${authority%%:*}"
case "$host" in
  ggoqouojrqloyyhuboob.supabase.co | fpgwbdmvvurxnanvxjld.supabase.co) ;; # Atmee's two storage projects
  *) die "not an upload link from atmee.ai: $host" ;;
esac
path="/${url#https://"$authority"/}"
case "$path" in
  /storage/v1/object/upload/sign/*) ;;
  *) die "not an upload link from atmee.ai (not a signed upload path): $url" ;;
esac

IFS=/ read -r -a parts <<< "$file"
for part in "${parts[@]}"; do
  case "$part" in
    .|..) ;;
    .*) die "refusing to upload a hidden file or anything in a hidden folder: $file" ;;
  esac
done
[ ! -L "$file" ] || die "refusing to upload a symlink; use the file itself: $file"
[ -f "$file" ] || die "no such file: $file"

resolved="$(cd "$(dirname "$file")" && pwd -P)/$(basename "$file")"
root="$(pwd -P)"
case "$resolved" in
  "$root"/*) ;;
  *) die "file is outside the project; copy it into the project folder first: $file" ;;
esac

ext="$(printf '%s' "${file##*.}" | tr '[:upper:]' '[:lower:]')"
case "$ext" in
  jpg|jpeg) type=image/jpeg ;;
  png) type=image/png ;;
  webp) type=image/webp ;;
  wav) type=audio/wav ;;
  mp3) type=audio/mpeg ;;
  m4a) type=audio/mp4 ;;
  aac) type=audio/aac ;;
  ogg) type=audio/ogg ;;
  flac) type=audio/flac ;;
  opus) type=audio/opus ;;
  webm) type=audio/webm ;;
  pdf) type=application/pdf ;;
  docx) type=application/vnd.openxmlformats-officedocument.wordprocessingml.document ;;
  doc) type=application/msword ;;
  txt) type=text/plain ;;
  md) type=text/markdown ;;
  html) type=text/html ;;
  csv) type=text/csv ;;
  *) die "unsupported file type .$ext (portraits: jpg, png, webp; voice: mp3, wav, m4a, aac, ogg, flac, opus, webm; documents: pdf, docx, doc, txt, md, html, csv)" ;;
esac

size="$(wc -c < "$file" | tr -d ' ')"
# The server's answer is kept for the error, so a refused type or size says why.
answer="$(curl -sS --fail-with-body -X PUT -H "Content-Type: $type" --data-binary "@$file" "$url" 2>&1)" \
  || die "upload failed for $file ($type): $answer"
printf 'uploaded %s (%s bytes, %s)\n' "$file" "$size" "$type"

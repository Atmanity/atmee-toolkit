#!/usr/bin/env bash
# Build a mouth-crop contact sheet from v2 candidate previews, the same crop as
# the atmee CLI's `candidates sheet`: one row per candidate, twelve frames of
# the mouth area across the clip.
#
#   contact-sheet.sh <out.png> <label>=<preview-url-or-file> ...
#
# The crop is the lower middle of the frame, where the mouth sits in a
# head-and-shoulders picture. For other framing (a half-body v2 portrait puts
# the mouth higher), set MOUTH_TOP to the top of the band as a fraction of the
# height (default 0.58; the band is a quarter of the height tall), e.g.
# MOUTH_TOP=0.28 contact-sheet.sh ...
#
# Rows follow the order given; the row labels are printed, not drawn. Each
# row line also carries motion=<n>: the mean frame-to-frame change of the
# mouth crop (0 for a frozen mouth), to read across rows, not as a threshold.
set -euo pipefail

die() { printf 'contact-sheet.sh: %s\n' "$1" >&2; exit 1; }

[ $# -ge 2 ] || die "usage: contact-sheet.sh <out.png> <label>=<preview-url-or-file> ..."
command -v ffmpeg >/dev/null || die "needs ffmpeg on PATH (brew install ffmpeg); without it, give the creator the preview links instead"
command -v ffprobe >/dev/null || die "needs ffprobe on PATH (it comes with ffmpeg)"

out="$1"
shift
frames=12
top="${MOUTH_TOP:-0.58}"
case "$top" in
  0.[0-9]|0.[0-9][0-9]) awk -v t="$top" 'BEGIN { exit !(t <= 0.75) }' || die "MOUTH_TOP must be between 0 and 0.75, got $top" ;;
  *) die "MOUTH_TOP must be a fraction like 0.58, got $top" ;;
esac
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

rows=()
labels=()
i=0
for pair in "$@"; do
  label="${pair%%=*}"
  src="${pair#*=}"
  [ -n "$label" ] && [ -n "$src" ] && [ "$label" != "$pair" ] || die "expected <label>=<preview>, got: $pair"
  i=$((i + 1))
  case "$src" in
    https://*|http://*)
      clip="$tmp/c$i.mp4"
      curl -sS --fail --max-time 60 -o "$clip" "$src" || die "could not download the preview for $label" ;;
    *)
      [ -f "$src" ] || die "no such file for $label: $src"
      clip="$src" ;;
  esac
  seconds="$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$clip" 2>/dev/null || true)"
  rate="$(awk -v s="${seconds:-0}" -v f="$frames" 'BEGIN { s += 0; if (s > 0) printf "%.4f", f / s; else print 2 }')"
  row="$tmp/r$i.png"
  ffmpeg -loglevel error -y -i "$clip" \
    -vf "fps=$rate,crop=iw*0.5:ih*0.25:iw*0.25:ih*$top,scale=96:48,tile=${frames}x1" \
    -frames:v 1 "$row" || die "ffmpeg failed on the preview for $label"
  motion="$(ffmpeg -loglevel error -nostats -i "$clip" \
    -vf "fps=$rate,crop=iw*0.5:ih*0.25:iw*0.25:ih*$top,scale=96:48,format=gray,tblend=all_mode=difference,signalstats,metadata=print:key=lavfi.signalstats.YAVG:file=-" \
    -f null - 2>/dev/null | LC_ALL=C awk -F= '/YAVG/ { n++; if (n > 1) s += $2 } END { if (n > 1) printf "%.1f", s / (n - 1); else print "n/a" }')"
  rows+=("$row")
  labels+=("$label motion=$motion")
done

if [ "${#rows[@]}" -eq 1 ]; then
  cp "${rows[0]}" "$out"
else
  inputs=()
  for r in "${rows[@]}"; do inputs+=(-i "$r"); done
  ffmpeg -loglevel error -y "${inputs[@]}" -filter_complex "vstack=inputs=${#rows[@]}" "$out" \
    || die "ffmpeg could not stack the rows"
fi

printf 'wrote %s\n' "$out"
for n in "${!labels[@]}"; do printf 'row %d: %s\n' "$((n + 1))" "${labels[$n]}"; done

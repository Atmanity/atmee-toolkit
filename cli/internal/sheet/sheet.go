// Package sheet draws a v2 candidate contact sheet: one row per candidate,
// mouth crops over time. A row whose lips never part is a dead candidate.
//
// Frames come from ffmpeg (it must be on PATH). Candidate clips are centred
// portraits, so the mouth is cropped from a fixed lower-middle region rather
// than found by face detection.
package sheet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Frames per row, spread evenly over the clip (up to ~30 s for a v2 preview).
const Frames = 12

// mouthFilter samples Frames frames evenly over a clip of the given length,
// crops the lower-middle of each, and tiles the crops into one row.
func mouthFilter(seconds float64) string {
	rate := 2.0
	if seconds > 0 {
		rate = float64(Frames) / seconds
	}
	return fmt.Sprintf("fps=%.4f,crop=iw*0.5:ih*0.25:iw*0.25:ih*0.58,scale=96:48,tile=%dx1", rate, Frames)
}

// duration asks ffprobe for the clip length (0 when unknown).
func duration(ctx context.Context, clip string) float64 {
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	out, err := exec.CommandContext(ctx, probe, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", clip).Output()
	if err != nil {
		return 0
	}
	var d float64
	_, _ = fmt.Sscan(string(bytes.TrimSpace(out)), &d)
	return d
}

// Row is one candidate.
type Row struct {
	Label string
	URL   string
}

// ErrNoFFmpeg means ffmpeg is not installed.
var ErrNoFFmpeg = errors.New("`atmee avatars candidates sheet` needs ffmpeg on PATH (brew install ffmpeg)")

// Build downloads every row's clip and returns the sheet as PNG.
func Build(ctx context.Context, rows []Row) ([]byte, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, ErrNoFFmpeg
	}
	dir, err := os.MkdirTemp("", "atmee-sheet-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	const labelW = 90
	var strips []image.Image
	for i, r := range rows {
		clip := filepath.Join(dir, fmt.Sprintf("c%d.mp4", i))
		if err := download(ctx, r.URL, clip); err != nil {
			return nil, fmt.Errorf("candidate %s: %w", r.Label, err)
		}
		out := filepath.Join(dir, fmt.Sprintf("c%d.png", i))
		cmd := exec.CommandContext(ctx, ffmpeg, "-loglevel", "error", "-y", "-i", clip, "-vf", mouthFilter(duration(ctx, clip)), "-frames:v", "1", out)
		if msg, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("candidate %s: ffmpeg: %v %s", r.Label, err, bytes.TrimSpace(msg))
		}
		f, err := os.Open(out)
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		strips = append(strips, img)
	}
	if len(strips) == 0 {
		return nil, errors.New("no candidates to draw")
	}
	w, h := 0, 0
	for _, s := range strips {
		w = max(w, s.Bounds().Dx())
		h += s.Bounds().Dy()
	}
	sheet := image.NewRGBA(image.Rect(0, 0, labelW+w, h))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	y := 0
	for i, s := range strips {
		draw.Draw(sheet, image.Rect(labelW, y, labelW+s.Bounds().Dx(), y+s.Bounds().Dy()), s, s.Bounds().Min, draw.Src)
		d := font.Drawer{Dst: sheet, Src: image.NewUniform(color.White), Face: basicfont.Face7x13,
			Dot: fixed.P(6, y+s.Bounds().Dy()/2+5)}
		d.DrawString(rows[i].Label)
		y += s.Bounds().Dy()
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func download(ctx context.Context, url, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("preview download: %s", resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

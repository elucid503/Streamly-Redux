package subdl

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrNoSubtitleFile = errors.New("subdl: archive contained no subtitle file")

var timingLine = regexp.MustCompile(`(\d{1,2}:\d{2}:\d{2}),(\d{1,3})\s*-->\s*(\d{1,2}:\d{2}:\d{2}),(\d{1,3})`)

type zipCandidate struct {

	Name string
	Payload []byte

}

// SubDL returns SRT and ZIP; browsers take neither, so this is the one content transform in the system.
func toWebVTT(raw []byte, season, episode int) ([]byte, error) {

	if len(raw) >= 4 && raw[0] == 'P' && raw[1] == 'K' {

		unpacked, err := extractFromZip(raw, season, episode)

		if err != nil {

			return nil, err

		}

		raw = unpacked

	} else if !looksLikeSubtitle(raw) {

		return nil, ErrNoSubtitleFile

	} else if !looksEnglishSubtitle(raw) {

		return nil, ErrNoSubtitleFile

	}

	text := strings.TrimSpace(decodeText(raw))

	if strings.HasPrefix(text, "WEBVTT") {

		return []byte(text + "\n"), nil

	}

	converted := timingLine.ReplaceAllStringFunc(text, func(match string) string {

		parts := timingLine.FindStringSubmatch(match)

		if len(parts) != 5 {

			return match

		}

		return parts[1] + "." + padMs(parts[2]) + " --> " + parts[3] + "." + padMs(parts[4])

	})

	return []byte("WEBVTT\n\n" + strings.ReplaceAll(converted, "\r\n", "\n") + "\n"), nil

}

func padMs(value string) string {

	if len(value) >= 3 {

		return value[:3]

	}

	return value + strings.Repeat("0", 3-len(value))

}

func extractFromZip(data []byte, season, episode int) ([]byte, error) {

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	if err != nil {

		return nil, fmt.Errorf("subdl: unreadable archive: %w", err)

	}

	var episodeMatches []zipCandidate
	var fallback []zipCandidate

	for _, file := range archive.File {

		ext := strings.ToLower(filepath.Ext(file.Name))

		switch ext {

		case ".srt", ".vtt", ".ass", ".ssa":

		default:

			continue

		}

		if !looksEnglishName(file.Name) {

			continue

		}

		opened, err := file.Open()

		if err != nil {

			continue

		}

		payload, err := io.ReadAll(io.LimitReader(opened, maxBodyBytes))
		opened.Close()

		if err != nil || len(payload) == 0 {

			continue

		}

		candidate := zipCandidate{Name: file.Name, Payload: payload}

		if season > 0 && episode > 0 && nameMatchesEpisode(file.Name, season, episode) {

			episodeMatches = append(episodeMatches, candidate)
			continue

		}

		if episode > 0 && nameMatchesEpisode(file.Name, 0, episode) {

			episodeMatches = append(episodeMatches, candidate)
			continue

		}

		fallback = append(fallback, candidate)

	}

	if payload := pickZipCandidate(episodeMatches); payload != nil {

		return payload, nil

	}

	if payload := pickZipCandidate(fallback); payload != nil {

		return payload, nil

	}

	return nil, ErrNoSubtitleFile

}

func pickZipCandidate(candidates []zipCandidate) []byte {

	for _, candidate := range candidates {

		if looksEnglishSubtitle(candidate.Payload) {

			return candidate.Payload

		}

	}

	return nil

}

func looksLikeSubtitle(data []byte) bool {

	limit := len(data)

	if limit > 512 {

		limit = 512

	}

	text := strings.ToLower(string(data[:limit]))
	trimmed := strings.TrimSpace(text)

	return strings.Contains(text, "-->") || strings.HasPrefix(trimmed, "webvtt") || strings.HasPrefix(trimmed, "[script info]")

}

// Subtitle releases are frequently Windows-1252 rather than UTF-8, which would otherwise render as replacement characters.
func decodeText(raw []byte) string {

	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	if utf8.Valid(raw) {

		return string(raw)

	}

	runes := make([]rune, 0, len(raw))

	for _, b := range raw {

		runes = append(runes, rune(b))

	}

	return string(runes)

}

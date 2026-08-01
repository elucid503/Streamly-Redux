package subdl

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrNoSubtitleFile = errors.New("subdl: archive contained no subtitle file")

var timingLine = regexp.MustCompile(`(\d{1,2}:\d{2}:\d{2}),(\d{1,3})\s*-->\s*(\d{1,2}:\d{2}:\d{2}),(\d{1,3})`)

// SubDL returns SRT and ZIP; browsers take neither, so this is the one content transform in the system.
func toWebVTT(raw []byte) ([]byte, error) {

	if bytes.HasPrefix(raw, []byte("PK\x03\x04")) {

		unpacked, err := unzip(raw)

		if err != nil {

			return nil, err

		}

		raw = unpacked

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

func unzip(raw []byte) ([]byte, error) {

	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))

	if err != nil {

		return nil, fmt.Errorf("subdl: unreadable archive: %w", err)

	}

	for _, file := range archive.File {

		name := strings.ToLower(file.Name)

		if !strings.HasSuffix(name, ".srt") && !strings.HasSuffix(name, ".vtt") {

			continue

		}

		reader, err := file.Open()

		if err != nil {

			return nil, err

		}

		defer reader.Close()

		return io.ReadAll(io.LimitReader(reader, maxBodyBytes))

	}

	return nil, ErrNoSubtitleFile

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

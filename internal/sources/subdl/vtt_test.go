package subdl

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

const sampleSRT = "1\r\n00:00:01,500 --> 00:00:04,000\r\nI must not fear.\r\n\r\n2\r\n00:01:02,250 --> 00:01:03,100\r\nFear is the mind-killer.\r\n"

func TestSRTBecomesWebVTT(t *testing.T) {

	out, err := toWebVTT([]byte(sampleSRT))

	if err != nil {

		t.Fatalf("conversion failed: %v", err)

	}

	text := string(out)

	if !strings.HasPrefix(text, "WEBVTT\n\n") {

		t.Fatalf("output did not start with a WEBVTT header: %q", text[:min(len(text), 20)])

	}

	// A browser rejects the comma that SRT uses for fractions.
	if strings.Contains(text, ",") {

		t.Fatalf("comma timings survived conversion: %q", text)

	}

	if !strings.Contains(text, "00:00:01.500 --> 00:00:04.000") {

		t.Fatalf("timing line was not rewritten: %q", text)

	}

	if !strings.Contains(text, "Fear is the mind-killer.") {

		t.Fatal("cue text was lost")

	}

}

func TestExistingWebVTTIsLeftAlone(t *testing.T) {

	out, err := toWebVTT([]byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nAlready fine.\n"))

	if err != nil {

		t.Fatalf("conversion failed: %v", err)

	}

	if strings.Count(string(out), "WEBVTT") != 1 {

		t.Fatalf("header was duplicated: %q", string(out))

	}

}

func TestArchiveIsUnpacked(t *testing.T) {

	buffer := &bytes.Buffer{}

	archive := zip.NewWriter(buffer)

	// Archives routinely carry a readme alongside the subtitle.
	notes, _ := archive.Create("readme.txt")

	_, _ = notes.Write([]byte("ignore me"))

	entry, _ := archive.Create("Dune.2021.1080p.srt")

	_, _ = entry.Write([]byte(sampleSRT))

	if err := archive.Close(); err != nil {

		t.Fatalf("building the archive failed: %v", err)

	}

	out, err := toWebVTT(buffer.Bytes())

	if err != nil {

		t.Fatalf("conversion failed: %v", err)

	}

	if !strings.Contains(string(out), "I must not fear.") {

		t.Fatalf("subtitle was not taken from the archive: %q", string(out))

	}

}

func TestArchiveWithoutASubtitleIsReported(t *testing.T) {

	buffer := &bytes.Buffer{}

	archive := zip.NewWriter(buffer)

	notes, _ := archive.Create("readme.txt")

	_, _ = notes.Write([]byte("nothing here"))

	_ = archive.Close()

	if _, err := toWebVTT(buffer.Bytes()); err == nil {

		t.Fatal("an archive with no subtitle converted without complaint")

	}

}

// Releases are frequently Windows-1252, which would otherwise render as replacement characters.
func TestNonUTF8TextSurvives(t *testing.T) {

	out, err := toWebVTT([]byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9\n"))

	if err != nil {

		t.Fatalf("conversion failed: %v", err)

	}

	if !strings.Contains(string(out), "Café") {

		t.Fatalf("latin-1 text was mangled: %q", string(out))

	}

}

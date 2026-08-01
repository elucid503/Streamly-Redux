package tmdb

import "testing"

func TestRawTitleUsesTVFieldsAndBackdrop(t *testing.T) {

	raw := rawTitle{

		ID: 42,

		Name:         "Example Show",
		FirstAirDate: "2024-03-12",

		PosterPath:   "/poster.jpg",
		BackdropPath: "/backdrop.jpg",

		VoteAverage: 8.25,
	}

	title := raw.title(KindTV)

	if title.Title != "Example Show" || title.Year != 2024 {

		t.Fatalf("parsed title %#v", title)

	}

	if title.Poster != imageURL+"/w500/poster.jpg" {

		t.Fatalf("poster %q", title.Poster)

	}

	if title.Backdrop != imageURL+"/w1280/backdrop.jpg" {

		t.Fatalf("backdrop %q", title.Backdrop)

	}

	if title.Rating != "8.2" {

		t.Fatalf("rating %q", title.Rating)

	}

}

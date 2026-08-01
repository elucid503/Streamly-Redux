package catalog

import (
	"testing"
)

func testIndex() *index {

	return newIndex([]reference{

		{ID: "ABC.us", Name: "ABC", Country: "US"},
		{ID: "KATV71.us", Name: "KATV 7.1", Country: "US", AltNames: []string{"ABC"}},
		{ID: "KGNSTV82.us", Name: "KGNS-TV 8.2", Country: "US", AltNames: []string{"ABC"}},

		{ID: "AMC.us", Name: "AMC", Country: "US"},
		{ID: "AMCPlus.us", Name: "AMC+", Country: "US"},

		{ID: "ESPN.us", Name: "ESPN", Country: "US"},
		{ID: "ESPN.nl", Name: "ESPN", Country: "NL"},

		{ID: "AnimalPlanet.de", Name: "Animal Planet", Country: "DE"},
		{ID: "AnimalPlanet.fr", Name: "Animal Planet", Country: "FR"},

		{ID: "AHC.us", Name: "American Heroes Channel", Country: "US", AltNames: []string{"AHC"}},

		{ID: "SkySportsMainEvent.uk", Name: "Sky Sports Main Event", Country: "UK"},
		{ID: "SkySportsMainEvent.ie", Name: "Sky Sports Main Event", Country: "IE"},

		{ID: "Setanta.ie", Name: "Setanta Sports", Country: "IE"},
		{ID: "SetantaPlus.ru", Name: "Setanta Sports+", Country: "RU"},

	})

}

func TestPrimaryNameOutranksAffiliateAltNames(t *testing.T) {

	found, ok := testIndex().match("abc usa")

	if !ok || found.reference.ID != "ABC.us" {

		t.Fatalf("matched %q (ok=%v), want ABC.us — the affiliates carry ABC as an alt name", found.reference.ID, ok)

	}

}

func TestExactNameOutranksNormalisedCollision(t *testing.T) {

	found, ok := testIndex().match("amc usa")

	if !ok || found.reference.ID != "AMC.us" {

		t.Fatalf("matched %q (ok=%v), want AMC.us — AMC+ normalises to the same key", found.reference.ID, ok)

	}

}

func TestCountryTokenSeparatesIdenticalNames(t *testing.T) {

	found, ok := testIndex().match("espn usa")

	if !ok || found.reference.ID != "ESPN.us" {

		t.Fatalf("matched %q (ok=%v), want ESPN.us", found.reference.ID, ok)

	}

	if found.assumed {

		t.Fatal("a country read off the title was reported as assumed")

	}

}

// iptv-org labels the United Kingdom "UK"; provider titles say "uk" and ISO would say "GB".
func TestUnitedKingdomMatchesEitherCode(t *testing.T) {

	index := newIndex([]reference{{ID: "SkySportsF1.uk", Name: "Sky Sports F1", Country: "UK"}})

	if _, ok := index.match("sky sports f1 uk"); !ok {

		t.Fatal("a uk-tagged title did not reach a UK-coded reference")

	}

}

// The same channel carried in several countries is guessable, and the guess is flagged so it can be counted.
func TestSameChannelInSeveralCountriesTakesThePreferredOne(t *testing.T) {

	found, ok := testIndex().match("sky sports main event")

	if !ok || found.reference.ID != "SkySportsMainEvent.uk" {

		t.Fatalf("matched %q (ok=%v), want SkySportsMainEvent.uk", found.reference.ID, ok)

	}

	if !found.assumed {

		t.Fatal("a country taken from the preference list was not reported as assumed")

	}

}

func TestPreferenceDoesNotReachOutsideItsList(t *testing.T) {

	found, ok := testIndex().match("animal planet")

	if ok {

		t.Fatalf("matched %q, want a refusal — neither preferred country is present", found.reference.ID)

	}

}

// Two different channels colliding on a normalised key must never be guessed between.
func TestDifferentChannelsSharingAKeyStayRefused(t *testing.T) {

	found, ok := testIndex().match("setantasports")

	if ok {

		t.Fatalf("matched %q, want a refusal — Setanta Sports and Setanta Sports+ are different channels", found.reference.ID)

	}

}

func TestParentheticalIsTriedOnItsOwn(t *testing.T) {

	found, ok := testIndex().match("ahc (american heroes channel)")

	if !ok || found.reference.ID != "AHC.us" {

		t.Fatalf("matched %q (ok=%v), want AHC.us", found.reference.ID, ok)

	}

}

func TestUnknownTitleIsDropped(t *testing.T) {

	if found, ok := testIndex().match("bein sports mena english 1"); ok {

		t.Fatalf("matched %q, want a refusal", found.reference.ID)

	}

}

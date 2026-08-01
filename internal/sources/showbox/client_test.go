package showbox

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {

	return f(request)

}

func TestCallRetriesTruncatedJSON(t *testing.T) {

	attempts := 0

	client := &Client{

		http: &http.Client{

			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {

				attempts++

				body := `{"code":1,"data":`

				if attempts == 2 {

					body = `{"code":1,"data":["one"]}`

				}

				return &http.Response{

					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil

			}),
		},
	}

	var result []string

	if err := client.call(context.Background(), "Search5", nil, &result); err != nil {

		t.Fatal(err)

	}

	if attempts != 2 {

		t.Fatalf("made %d attempts, want 2", attempts)

	}

	if len(result) != 1 || result[0] != "one" {

		t.Fatalf("decoded %#v", result)

	}

}

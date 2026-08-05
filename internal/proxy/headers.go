package proxy

import (
	"net/http"
	"net/url"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

func applyHeaders(req *http.Request, source string, referer *url.URL) {

	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")

	if referer != nil {

		req.Header.Set("Referer", referer.String())
		req.Header.Set("Origin", referer.Scheme+"://"+referer.Host)

	}

	switch source {

	case "daddylive":

		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	case "ntv":

		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		if referer == nil {

			req.Header.Set("Referer", "https://cdnlivetv.tv/")
			req.Header.Set("Origin", "https://cdnlivetv.tv")

		}

	}

}

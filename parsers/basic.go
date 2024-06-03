package parsers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/charmbracelet/log"
	"golang.org/x/net/html"
)

type Basic struct {
}

func NewBasic() Basic {
	return Basic{}
}

func (p Basic) Fetch(url *url.URL) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(common.Options.RequestTimeout)
		log.Warn("connection timed out", "url", url)
		cancel()
	}()

	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	res.Body.Close()

	return bodyBytes, err
}

func findHref(z *html.Tokenizer) ([]byte, bool) {
	k, v, more := z.TagAttr()
	if string(k) == "href" {
		return v, true
	}

	if more {
		return findHref(z)
	}

	return nil, false
}

func (p Basic) ParsePage(data []byte, original *url.URL) (links []*url.URL, text []byte, err error) {
	z := html.NewTokenizer(bytes.NewReader(data))

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			newErr := z.Err()
			if errors.Is(newErr, io.EOF) {
				return
			}

			err = newErr
			return
		case html.TextToken:
			text = append(text, z.Text()...)
		case html.StartTagToken, html.EndTagToken:
			tn, _ := z.TagName()
			if len(tn) == 1 && tn[0] == 'a' {
				href, ok := findHref(z)
				if !ok {
					continue
				}

				url, err := url.Parse(string(href))
				if err != nil {
					log.Warn("could not parse url", "error", err, "href", string(href))
					continue
				}

				url = original.ResolveReference(url)

				if !slices.Contains([]string{"http", "https"}, url.Scheme) {
					log.Debug("url did not match scheme whitelist", "url", url)
					continue
				}

				url.RawQuery = ""
				url.RawFragment = ""
				url.Fragment = ""

				links = append(links, url)
			}
		}
	}
}

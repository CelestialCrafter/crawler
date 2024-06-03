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
	client *http.Client
	logger *log.Logger
}

func NewBasic() Basic {
	return Basic{
		client: &http.Client{},
		logger: log.WithPrefix("parser/basic"),
	}
}

func (p Basic) Fetch(u *url.URL) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan bool, 1)

	go func() {
		time.Sleep(common.Options.RequestTimeout)
		select {
		case <-finished:
			return
		default:
			p.logger.Warn("connection timed out", "url", u)
			cancel()
		}
	}()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	finished <- true

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
					p.logger.Warn("could not parse url", "error", err, "href", string(href))
					continue
				}

				url = original.ResolveReference(url)

				if !slices.Contains([]string{"http", "https"}, url.Scheme) {
					p.logger.Debug("url did not match scheme whitelist", "url", url)
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

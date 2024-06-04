package basic

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"slices"

	"golang.org/x/net/html"
)

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

func (p Basic) parseHtml(data []byte, original *url.URL) (links []*url.URL, text []byte, err error) {
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

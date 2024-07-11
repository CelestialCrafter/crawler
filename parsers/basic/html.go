package basic

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

var whitelistedTextTags = []string{
	"h1",
	"h2",
	"h3",
	"h4",
	"h5",
	"h6",
	"blockquote",
	"dd",
	"dl",
	"dt",
	"figcaption",
	"li",
	"p",
	"pre",
}

func findHrefSrc(z *html.Tokenizer) ([]byte, bool) {
	k, v, more := z.TagAttr()
	if slices.Contains([]string{"src", "href"}, string(k)) {
		return v, true
	}

	if more {
		return findHrefSrc(z)
	}

	return nil, false
}

// note to self: don't put a context canceled check on this function
// the time it takes to parse explodes
func (p Basic) parseHtml(data []byte, original *url.URL) (links []*url.URL, text []byte, err error) {
	z := html.NewTokenizer(bytes.NewReader(data))
	useText := false

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			newErr := z.Err()
			if errors.Is(newErr, io.EOF) {
				text = removeExtraWhitespace(text)
				return
			}

			err = newErr
			return
		case html.TextToken:
			if useText {
				text = append(text, z.Text()...)
			}
		case html.EndTagToken:
			tn, _ := z.TagName()
			if slices.Contains(whitelistedTextTags, string(tn)) {
				useText = false
			}
		case html.StartTagToken:
			tn, _ := z.TagName()

			if slices.Contains([]string{"a", "img"}, string(tn)) {
				link, ok := findHrefSrc(z)
				if !ok {
					continue
				}

				url, err := url.Parse(strings.TrimSpace(string(link)))
				if err != nil {
					p.logger.Warn("unable to parse url", "error", err, "href", string(link))
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
			} else if slices.Contains(whitelistedTextTags, string(tn)) {
				useText = true
			}
		}
	}
}

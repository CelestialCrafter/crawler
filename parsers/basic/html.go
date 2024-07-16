package basic

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	pb "github.com/CelestialCrafter/crawler/protos"
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

type attributeResult struct {
	k string
	v string
}

func findAttributes(z *html.Tokenizer, search []string) (attributes []attributeResult) {
	find := func() (*attributeResult, bool) {
		k, v, more := z.TagAttr()
		if slices.Contains(search, string(k)) {
			return &attributeResult{k: string(k), v: string(v)}, more
		}

		return nil, more
	}

	for {
		attr, more := find()

		if attr != nil {
			attributes = append(attributes, *attr)
		}

		if !more {
			break
		}
	}

	return
}

var whitespace = regexp.MustCompile(`\s+`)

// note to self: don't put a context canceled check on this function
// the time it takes to parse explodes
func (p Basic) parseHtml(data *pb.Document, original *url.URL) error {
	z := html.NewTokenizer(bytes.NewReader(data.Original))
	useText := false
	text := make([]byte, 0)
	links := make([]string, 0)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if errors.Is(err, io.EOF) {
				data.Text = whitespace.ReplaceAllLiteral(text, []byte(" "))
				data.Children = links
				return nil
			}

			return err
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
				newLinks := findAttributes(z, []string{"href", "src"})
				if len(newLinks) < 1 {
					continue
				}

				link := newLinks[0]

				u, err := url.Parse(strings.TrimSpace(link.v))
				if err != nil {
					p.logger.Warn("unable to parse url", "error", err, "href", link.v)
					continue
				}

				u = original.ResolveReference(u)

				if strings.Contains(u.Host, "wiki") {
					continue
				}

				if !slices.Contains([]string{"http", "https"}, u.Scheme) {
					p.logger.Debug("url did not match scheme whitelist", "url", u)
					continue
				}

				u.RawQuery = ""
				u.RawFragment = ""
				u.Fragment = ""

				links = append(links, u.String())
			} else if string(tn) == "meta" {
				metadataAttributes := findAttributes(z, []string{"name", "property", "content"})

				// make sure the name comes before content
				// so it can know which name goes with which value
				slices.SortFunc(
					metadataAttributes,
					func(a attributeResult, b attributeResult) int {
						if a.k == "content" {
							return 1
						}

						if b.k == "content" {
							return -1
						}

						return 0
					},
				)

				name := ""
				for _, attr := range metadataAttributes {
					k := attr.k
					v := attr.v
					if k == "name" || k == "property" {
						name = attr.v
					} else {
						switch name {
						case "description", "og:description":
							data.Metadata.Description = &v
						case "og:site_name":
							data.Metadata.Site = &v
						case "og:title":
							data.Metadata.Title = &v
						}
					}
				}
			} else if slices.Contains(whitelistedTextTags, string(tn)) {
				useText = true
			}
		}
	}
}

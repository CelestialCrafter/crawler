package parsers

import (
	"net/url"
)

type Parser interface {
	Fetch(link *url.URL) (data []byte, err error)
	ParsePage(data []byte, original *url.URL) (links []*url.URL, text []byte, err error)
}

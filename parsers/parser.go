package parsers

import (
	"context"
	"net/url"
)

type Parser interface {
	Fetch(link string, ctx context.Context) (data []byte, err error)
	ParsePage(data []byte, original *url.URL, ctx context.Context) (links []*url.URL, text []byte, err error)
}

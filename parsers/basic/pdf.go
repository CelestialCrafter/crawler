package basic

import (
	"errors"
	"net/url"
)

func (p Basic) parsePdf(_ []byte, _ *url.URL) (links []*url.URL, text []byte, err error) {
	return nil, nil, errors.New("PDF parsing tbh")
}

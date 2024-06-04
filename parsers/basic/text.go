package basic

import "net/url"

func (p Basic) parseText(data []byte, _ *url.URL) (links []*url.URL, text []byte, err error) {
	return nil, data, nil
}

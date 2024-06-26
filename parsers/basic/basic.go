package basic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/charmbracelet/log"
)

const MAX_MIME_BYTES = 24

type Basic struct {
	client *http.Client
	logger *log.Logger
}

func New() Basic {
	return Basic{
		client: &http.Client{},
		logger: log.WithPrefix("parser/basic"),
	}
}

func (p Basic) Fetch(u *url.URL, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", common.Options.UserAgent)

	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	contentType := res.Header.Get("content-type")
	contentType = strings.Split(contentType, ";")[0]
	if !slices.Contains([]string{
		"text/html",
		"text/plain",
		"application/pdf",
		"application/rtf",
	}, contentType) {
		return nil, fmt.Errorf("content type header did not match the mime type whitelist: %v", contentType)
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	res.Body.Close()

	contentTypeBytes := []byte(contentType)
	mime := make([]byte, MAX_MIME_BYTES)
	copy(mime, contentTypeBytes)

	return append(mime, bodyBytes...), err
}

func (p Basic) ParsePage(data []byte, original *url.URL, ctx context.Context) (links []*url.URL, text []byte, err error) {
	mime := string(bytes.Trim(data[:MAX_MIME_BYTES], "\x00"))
	data = data[MAX_MIME_BYTES:]

	switch mime {
	case "text/html":
		return p.parseHtml(data, original)
	case "text/plain":
		return p.parseText(data, original)
	case "text/markdown":
		return p.parseText(data, original)
	case "application/pdf":
		return p.parsePdf(data, original)
	case "application/rdf":
	default:
		p.parseHtml(data, original)
	}

	return nil, nil, fmt.Errorf("could not find parse mime type: %v", mime)
}

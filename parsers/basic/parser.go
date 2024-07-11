package basic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
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
	err := os.MkdirAll(path.Join(common.Options.DataPath, "tmp"), 0755)
	if err != nil {
		log.Fatal("unable to create DataPath/tmp", "error", err)
	}

	return Basic{
		client: &http.Client{},
		logger: log.WithPrefix("parser/basic"),
	}
}

func (p Basic) Fetch(u string, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", common.Options.UserAgent)

	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("recieved status code: %d", res.StatusCode)
	}

	contentType := res.Header.Get("content-type")
	if contentType == "" {
		// https://www.rfc-editor.org/rfc/rfc9110.html#section-8.3-5
		contentType = "application/octet-stream"
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	res.Body.Close()

	contentTypeBytes := []byte(strings.Split(contentType, ";")[0])
	mime := make([]byte, MAX_MIME_BYTES)
	copy(mime, contentTypeBytes)

	return append(mime, bodyBytes...), err
}

func (p Basic) ParsePage(data []byte, original *url.URL) (links []*url.URL, text []byte, err error) {
	m := string(bytes.Trim(data[:MAX_MIME_BYTES], "\x00"))
	data = data[MAX_MIME_BYTES:]

	switch m {
	case "text/html":
		return p.parseHtml(data, original)
	case "application/pdf":
		return p.parsePdf(data, original)
	case "text/plain", "text/markdown", "image/jpeg", "image/png", "image/webp", "image/gif":
		return p.parseUnchanged(data, original)
	}

	return nil, nil, fmt.Errorf("unable to find parser for mime type: %v", m)
}

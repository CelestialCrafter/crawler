package basic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CelestialCrafter/crawler/common"
	pb "github.com/CelestialCrafter/crawler/protos"
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

func (p Basic) Fetch(data *pb.Document, ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", data.Url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", common.Options.UserAgent)

	res, err := p.client.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("recieved status code: %d", res.StatusCode)
	}

	contentType := res.Header.Get("content-type")
	if contentType == "" {
		// https://www.rfc-editor.org/rfc/rfc9110.html#section-8.3-5
		contentType = "application/octet-stream"
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	res.Body.Close()

	mime := strings.Split(contentType, ";")[0]
	data.Original = bodyBytes
	data.Metadata.Mime = mime

	return nil
}

func (p Basic) ParsePage(data *pb.Document, original *url.URL) error {
	mime := data.Metadata.Mime

	switch mime {
	case "text/html":
		return p.parseHtml(data, original)
	case "application/pdf":
		return p.parsePdf(data, original)
	case "text/plain", "text/markdown", "image/jpeg", "image/png", "image/webp", "image/gif":
		return nil
	}

	return fmt.Errorf("unable to find parser for mime type: %v", mime)
}

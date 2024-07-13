package basic

import (
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/charmbracelet/log"
)

var arxivRegex = regexp.MustCompile(`abs/\d{4}\.\d+`)
var urlRegex = regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)

func findLinks(b []byte) []*url.URL {
	arxivsBytes := arxivRegex.FindAll(b, -1)
	linksBytes := arxivRegex.FindAll(b, -1)
	arxivs := make([]*url.URL, len(arxivsBytes))
	links := make([]*url.URL, len(linksBytes))

	for i, id := range arxivsBytes {
		u, err := url.Parse("https://arxiv.org/" + string(id))
		if err != nil {
			log.Warn("unable to parse url", "url", u)
		}

		arxivs[i] = u

	}

	for i, urlBytes := range linksBytes {
		u, err := url.Parse(string(urlBytes))
		if err != nil {
			log.Warn("unable to parse url", "url", u)
		}

		links[i] = u
	}

	return append(arxivs, links...)
}

func (p Basic) parsePdf(data []byte, original *url.URL) (links []*url.URL, text []byte, err error) {
	tmpdir := os.ExpandEnv("$TMPDIR")
	if tmpdir == "" {
		return nil, nil, errors.New("$TMPDIR variable not set")
	}

	path := path.Join(
		tmpdir,
		base64.StdEncoding.EncodeToString([]byte(original.String()))+".pdf",
	)

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return nil, nil, err
	}

	stdout, err := exec.Command("pdftotext", path, "-nopgbrk", "-").Output()
	if err != nil {
		return nil, nil, err
	}

	// dont care about error, its just tmp cleanup
	_ = os.Remove(path)

	return findLinks(stdout), stdout, nil
}

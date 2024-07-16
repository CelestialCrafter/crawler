package basic

import (
	"encoding/base64"
	"errors"
	pb "github.com/CelestialCrafter/crawler/protos"
	"github.com/charmbracelet/log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
)

var arxivRegex = regexp.MustCompile(`abs/\d{4}\.\d+`)
var urlRegex = regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)

func findLinks(b []byte) []string {
	arxivsBytes := arxivRegex.FindAll(b, -1)

	linksBytes := arxivRegex.FindAll(b, -1)
	arxivs := make([]string, len(arxivsBytes))
	links := make([]string, len(linksBytes))

	for i, id := range arxivsBytes {
		u, err := url.Parse("https://arxiv.org/" + string(id))
		if err != nil {
			log.Warn("unable to parse url", "url", u)
		}

		arxivs[i] = u.String()

	}

	for i, urlBytes := range linksBytes {
		u, err := url.Parse(string(urlBytes))
		if err != nil {
			log.Warn("unable to parse url", "url", u)
		}

		links[i] = u.String()
	}

	return append(arxivs, links...)
}

func (p Basic) parsePdf(data *pb.Document, original *url.URL) error {
	tmpdir := os.ExpandEnv("$TMPDIR")
	if tmpdir == "" {
		return errors.New("$TMPDIR variable not set")
	}

	path := path.Join(
		tmpdir,
		base64.StdEncoding.EncodeToString([]byte(original.String()))+".pdf",
	)

	err := os.WriteFile(path, data.Original, 0644)
	if err != nil {
		return err
	}

	stdout, err := exec.Command("pdftotext", path, "-nopgbrk", "-").Output()
	if err != nil {
		return err
	}

	// dont care about error, its just tmp cleanup
	_ = os.Remove(path)

	data.Text = stdout
	data.Children = findLinks(stdout)
	return nil
}

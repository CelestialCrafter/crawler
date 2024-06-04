package main

import (
	"archive/tar"
	"net/url"
	"os"

	"golang.org/x/exp/maps"

	"github.com/charmbracelet/log"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers/basic"
)

func stringsToUrls(urlsStrings []string) ([]*url.URL, error) {
	urls := make([]*url.URL, len(urlsStrings))
	for i, urlString := range urlsStrings {
		var err error
		urls[i], err = url.Parse(urlString)
		if err != nil {
			return nil, err
		}
	}

	return urls, nil
}

func urlsToBytes(urls []*url.URL) []byte {
	var bytes []byte
	for _, url := range urls {
		urlBytes := append([]byte(url.String()), byte('\n'))
		bytes = append(bytes, urlBytes...)
	}

	return bytes
}

func main() {
	_, err := common.LoadOptions()
	if err != nil {
		log.Fatal("unable to load options", "error", err)
	}

	// logging
	logger := log.NewWithOptions(os.Stderr, common.LogOptions)
	log.SetDefault(logger)

	// init stuff
	startingUrls, err := chooseStartUrls()
	if err != nil {
		log.Fatal("unable to get starting urls", "error", err)
	}

	if len(startingUrls) < 1 {
		log.Warn("no urls in starting urls")
	}

	crawledList, err := initializeCrawledList()
	if err != nil {
		log.Fatal("unable to initialize crawled list", "error", err)
	}

	cf, err := os.Create(common.Options.CrawledTarPath)
	if err != nil {
		log.Fatal("unable to open crawled tarball", "error", err)
	}
	defer cf.Close()

	cw := tar.NewWriter(cf)
	defer cw.Close()

	// crawl loop
	parser := basic.New()
	frontier := startingUrls

	for i := 0; i < common.Options.CrawlDepth || common.Options.CrawlDepth < 1; i++ {
		log.Info("running crawling itteration", "i", i)
		frontier = crawlItteration(frontier, parser, cw, &crawledList)

		err := os.WriteFile(common.Options.FrontierPath, urlsToBytes(maps.Keys(frontier)), 0644)
		if err != nil {
			log.Fatal("unable to write to frontier", "error", err)
		}

		err = os.WriteFile(common.Options.CrawledListPath, urlsToBytes(maps.Keys(crawledList)), 0644)
		if err != nil {
			log.Fatal("unable to write to crawled list", "error", err)
		}

		if len(frontier) < 1 {
			log.Warn("no links in new frontier; exiting.", "i", i)
			break
		}
	}
}

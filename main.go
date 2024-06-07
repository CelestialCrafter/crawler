package main

import (
	"archive/tar"
	"net/url"
	"os"
	"runtime/pprof"
	"time"

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
	common.RecalculateLogOptions()

	// profiling
	if common.Options.EnableProfiler {
		pf, err := os.Create("data/crawler.prof")
		if err != nil {
			log.Fatal("could not open profiler file", "error", err)
			return
		}

		pprof.StartCPUProfile(pf)
		defer pprof.StopCPUProfile()

	}

	// logging
	logger := log.NewWithOptions(os.Stderr, common.LogOptions)
	log.SetDefault(logger)

	// init stuff
	err = os.MkdirAll("data/robots", 0644)
	if err != nil {
		log.Fatal("unable to create data/ and/or data/robots/ directories", "error", err)
	}

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
	_ = parser

	for i := 0; i < common.Options.CrawlDepth || common.Options.CrawlDepth < 1; i++ {
		log.Info("running crawling itteration", "i", i)
		start := time.Now()
		frontier = crawlItteration(frontier, parser, cw, &crawledList)

		err := os.WriteFile(common.Options.FrontierPath, urlsToBytes(maps.Keys(frontier)), 0644)
		if err != nil {
			log.Fatal("unable to write to frontier", "error", err)
		}

		err = os.WriteFile(common.Options.CrawledListPath, urlsToBytes(maps.Keys(crawledList)), 0644)
		if err != nil {
			log.Fatal("unable to write to crawled list", "error", err)
		}

		log.Info("completed itteration", "duration", time.Since(start))

		if len(frontier) < 1 {
			log.Warn("no links in new frontier; exiting.", "i", i)
			break
		}
	}
}

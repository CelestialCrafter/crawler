package main

import (
	"archive/tar"
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"sync"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/charmbracelet/log"
)

type UrlsErr struct {
	original *url.URL
	urls     []*url.URL
	err      error
}

func crawl(u *url.URL, parser parsers.Parser, tw *tar.Writer) ([]*url.URL, error) {
	log.Info("crawling url", "url", u)

	htm, err := parser.Fetch(u)
	if err != nil {
		return nil, err
	}

	urls, text, err := parser.ParsePage(htm, u)
	if err != nil {
		return nil, err
	}

	encodedPath := base64.URLEncoding.EncodeToString([]byte(u.String()))
	writeTar(tw, fmt.Sprintf("%s/%s.txt", u.Hostname(), encodedPath), text)
	log.Info("wrote data to tar", "url", u)

	return urls, nil
}

func crawlItteration(frontier map[*url.URL]struct{}, parser parsers.Parser, tw *tar.Writer, crawledList *map[*url.URL]struct{}) map[*url.URL]struct{} {
	newFrontier := map[*url.URL]struct{}{}

	ch := make(chan UrlsErr, len(frontier))
	guard := make(chan struct{}, common.Options.MaxConcurrentCrawlers)
	var wg sync.WaitGroup

	for u, _ := range frontier {
		_, ok := (*crawledList)[u]
		if ok {
			log.Debug("already crawled url", "url", u)
		}

		wg.Add(1)
		guard <- struct{}{}
		go func(original *url.URL) {
			defer wg.Done()
			defer func() { <-guard }()
			urls, err := crawl(original, parser, tw)
			ch <- UrlsErr{
				original,
				urls,
				err,
			}
		}(u)
	}

	go func() {
		wg.Wait()
		close(ch)
		tw.Flush()
	}()

	for result := range ch {
		urls, err, original := result.urls, result.err, result.original
		if err != nil {
			log.Error("unable to crawl", "error", err, "url", original)
			continue
		}

		(*crawledList)[original] = struct{}{}
		filteredUrls := slices.DeleteFunc(urls, func(u *url.URL) bool {
			_, ok := (*crawledList)[u]
			return ok
		})

		for _, u := range filteredUrls {
			newFrontier[u] = struct{}{}
		}
	}

	return newFrontier
}

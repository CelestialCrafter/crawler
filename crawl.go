package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/benjaminestes/robots"
	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-metrics"
)

type UrlsErr struct {
	original *url.URL
	urls     []*url.URL
	err      error
}

var hostMutexMap = map[string]*sync.Mutex{}
var robotsMap = map[string]*robots.Robots{}
var crawlDelayMap = map[string]time.Time{}

func newCrawlLogger(u *url.URL) *log.Logger {
	return log.WithPrefix("crawler").With("url", u)
}

func encodeUrl(urlString string) string {
	return base64.URLEncoding.EncodeToString([]byte(urlString))
}

func crawlAllowed(u *url.URL, crawledList *map[string]struct{}, ctx context.Context) error {
	// already crawled check
	_, ok := (*crawledList)[u.String()]
	if ok {
		return errors.New("already crawled url")
	}

	crawlerLog := common.LoggerFromContext(ctx)

	// robots
	// @TODO ignore robots.txt data past 500kb (https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt#file-format)
	// @TODO use noindex (https://developers.google.com/search/docs/crawling-indexing/block-indexing)

	if !common.Options.RespectRobots {
		return nil
	}

	hostMutex := hostMutexMap[u.Host]
	hostMutex.Lock()
	defer hostMutex.Unlock()

	_, ok = robotsMap[u.Host]
	if !ok {
		start := time.Now()
		robotsLocation, err := robots.Locate(u.String())
		if err != nil {
			return err
		}
		res, err := http.Get(robotsLocation)
		if err != nil {
			return err
		}

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		res.Body.Close()

		// robots.From will panic if the length is 0, and < 1 didnt work so im adding a little minimum
		if len(bodyBytes) < 10 {
			return nil
		}

		robots, err := robots.From(res.StatusCode, bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}

		robotsMap[u.Host] = robots
		crawlerLog.Debug("fetched robots.txt", "duration", time.Since(start))
	}

	if !robotsMap[u.Host].Test(common.Options.UserAgent, u.String()) {
		return errors.New("url was disalowed by robots")
	}

	return nil
}

func sleepUntilCrawlable(u *url.URL, ctx context.Context) {
	// @TODO respect robots.txt Crawl-delay
	crawlerLog := common.LoggerFromContext(ctx)

	hostMutex := hostMutexMap[u.Host]
	hostMutex.Lock()

	crawlDelay, ok := crawlDelayMap[u.Host]
	comparison := time.Now().Compare(crawlDelay) >= 0

	// @FIX possible bug here? it sleeps and then locks until an already passed time
	if !ok || comparison {
		crawlDelayMap[u.Host] = time.Now().Add(common.Options.DefaultCrawlDelay)
	}

	crawlDelay = crawlDelayMap[u.Host]
	timeUntil := time.Until(crawlDelay)
	if ok && !comparison {
		time.Sleep(timeUntil)
	}

	// lock host mutex till crawl delay finishes
	go func() {
		crawlerLog.Debug("locking host until crawl delay", "host", u.Host, "delay", timeUntil)
		time.Sleep(timeUntil)
		hostMutex.Unlock()
	}()
}

func crawl(u *url.URL, parser parsers.Parser, tw *tar.Writer, ctx context.Context) ([]*url.URL, error) {
	start := time.Now()
	crawlerLog := common.LoggerFromContext(ctx)

	htm, err := parser.Fetch(u, ctx)
	if err != nil {
		return nil, err
	}
	metrics.MeasureSince([]string{"fetch"}, start)
	kb := float32(len(htm)) / 1000

	parseStart := time.Now()
	urls, text, err := parser.ParsePage(htm, u, ctx)
	if err != nil {
		return nil, err
	}
	metrics.MeasureSince([]string{"parse"}, parseStart)

	// @TODO change .txt to match the mime of the page (support .pdfs, .jpg/.png, ect)
	writeTar(tw, fmt.Sprintf("%s/%s.txt", u.Hostname(), encodeUrl(u.String())), text)
	crawlerLog.Debug("crawled page", "urls", len(urls), "kb", kb, "duration", time.Since(start))

	// @TODO use a domain label when you fix your metrics... stupid..
	metrics.IncrCounter([]string{"crawled_count"}, 1)

	return urls, nil
}

func crawlItteration(frontier map[*url.URL]struct{}, parser parsers.Parser, tw *tar.Writer, crawledList *map[string]struct{}) map[*url.URL]struct{} {
	// @TODO use sitemap from robots.txt
	newFrontier := map[*url.URL]struct{}{}

	ch := make(chan UrlsErr, len(frontier))
	guard := make(chan struct{}, common.Options.MaxConcurrentCrawlers)
	var wg sync.WaitGroup

	for u := range frontier {
		_, ok := hostMutexMap[u.Host]
		if !ok {
			hostMutexMap[u.Host] = &sync.Mutex{}
		}

		crawlerLog := newCrawlLogger(u)
		ctx := context.WithValue(context.Background(), common.ContextLogger, crawlerLog)

		err := crawlAllowed(u, crawledList, ctx)
		if err != nil {
			crawlerLog.Debug("url was not allowed to be crawled", "error", err)
		}

		ctx, cancel := context.WithTimeout(ctx, common.Options.CrawlTimeout)
		sleepUntilCrawlable(u, ctx)

		wg.Add(1)
		guard <- struct{}{}

		go func(original *url.URL, cancel *context.CancelFunc) {
			defer func() {
				wg.Done()
				<-guard
				(*cancel)()
			}()

			urls, err := crawl(original, parser, tw, ctx)
			ch <- UrlsErr{
				original,
				urls,
				err,
			}
		}(u, &cancel)
	}

	go func() {
		wg.Wait()
		close(ch)
		tw.Flush()
	}()

	for result := range ch {
		// _ = result
		urls, err, original := result.urls, result.err, result.original
		if err != nil {
			newCrawlLogger(original).Error("unable to crawl", "error", err)
			continue
		}

		(*crawledList)[original.String()] = struct{}{}
		for _, u := range urls {
			newFrontier[u] = struct{}{}
		}
	}

	return newFrontier
}

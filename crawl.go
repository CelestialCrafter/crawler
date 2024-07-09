package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-metrics"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/temoto/robotstxt"
)

// global maps
var robotsMap = xsync.NewMapOf[string, *robotstxt.Group]()
var crawlDelayMap = xsync.NewMapOf[string, time.Time]()

func newCrawlLogger(u string, worker int) *log.Logger {
	return log.WithPrefix("crawler").With("url", u, "worker", worker)
}

func encodeUrl(urlString string) string {
	return base64.URLEncoding.EncodeToString([]byte(urlString))
}

func fetchRobots(u *url.URL, ctx context.Context) (*robotstxt.Group, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprint(u.Scheme, "://", u.Host, "/robots.txt"), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", common.Options.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	// @TODO support multi port robots
	defer resp.Body.Close()
	robots, err := robotstxt.FromResponse(resp)
	if err != nil {
		return nil, err
	}

	group := robots.FindGroup(common.Options.UserAgent)

	return group, nil
}

func crawlAllowed(u *url.URL, ctx context.Context) error {
	// @FIX ignore robots.txt data past 500kb (https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt#file-format)
	// @TODO use noindex maybe? (https://developers.google.com/search/docs/crawling-indexing/block-indexing)
	if !common.Options.RespectRobots {
		return nil
	}

	crawlerLog := common.LoggerFromContext(ctx)

	robotsHost, _ := robotsMap.LoadOrCompute(u.Host, func() *robotstxt.Group {
		start := time.Now()
		g, err := fetchRobots(u, ctx)
		if err != nil {
			crawlerLog.Error("unable to fetch robots.txt", "error", err, "duration", time.Since(start))
			return &robotstxt.Group{}
		}

		crawlerLog.Debug("fetched robots.txt", "duration", time.Since(start))
		return g
	})

	if !robotsHost.Test(u.String()) {
		return errors.New("url was disalowed by robots")
	}

	return nil
}

func worker(i int, queue *[]*url.URL, parser parsers.Parser, writer func([]string, string, []byte)) {
	for len(*queue) > 0 {
		u := (*queue)[0]
		urlString := u.String()
		*queue = (*queue)[1:]

		crawlerLog := newCrawlLogger(urlString, i)
		ctx := context.WithValue(context.Background(), common.ContextLogger, crawlerLog)

		err := func(u *url.URL) error {
			if common.Options.DefaultCrawlDelay != time.Second*0 {
				var oldCrawlTime time.Time
				crawlDelayMap.Compute(
					u.Host,
					func(oldValue time.Time, loaded bool) (newValue time.Time, delete bool) {
						delete = false
						oldCrawlTime = oldValue

						var startingPoint time.Time
						now := time.Now()

						if oldValue.After(now) {
							startingPoint = oldValue
						} else {
							startingPoint = now
						}

						newValue = startingPoint.Add(common.Options.DefaultCrawlDelay)

						return
					},
				)

				if time.Now().Before(oldCrawlTime) {
					time.Sleep(time.Until(oldCrawlTime))
				}
			}

			ctx, cancel := context.WithTimeout(ctx, common.Options.CrawlTimeout)
			defer cancel()

			err := crawlAllowed(u, ctx)
			if err != nil {
				return err
			}

			fetchStart := time.Now()
			htm, err := parser.Fetch(urlString, ctx)
			if err != nil {
				return err
			}
			metrics.MeasureSince([]string{"fetch"}, fetchStart)
			kb := float32(len(htm)) / 1000

			urls, text, err := parser.ParsePage(htm, u)
			if err != nil {
				return err
			}

			urlsString := make([]string, len(urls))
			for i, u := range urls {
				urlsString[i] = u.String()
			}

			// @TODO change .txt to match the mime of the page (support .pdfs, .jpg/.png, ect)
			writer(urlsString, fmt.Sprintf("%s/%s.txt", u.Host, encodeUrl(urlString)), text)

			crawlerLog.Debug("crawled page", "urls", len(urls), "kb", kb, "duration", time.Since(fetchStart))

			return nil
		}(u)

		if err != nil {
			crawlerLog.Error("unable to crawl website", "error", err)
		}
	}
}

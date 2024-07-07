package main

import (
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
	"github.com/puzpuzpuz/xsync/v3"
)

// global maps
type robotsMutex struct {
	robots *robots.Robots
	mutex  *sync.Mutex
}

var robotsMap = map[string]robotsMutex{}
var crawlDelayMap = xsync.NewMapOf[string, time.Time]()

func preflight(batch []*url.URL) {
	for _, u := range batch {
		robotsMap[u.Hostname()] = robotsMutex{
			mutex: new(sync.Mutex),
		}
	}
}

func newCrawlLogger(u string, worker int) *log.Logger {
	return log.WithPrefix("crawler").With("url", u, "worker", worker)
}

func encodeUrl(urlString string) string {
	return base64.URLEncoding.EncodeToString([]byte(urlString))
}

func fetchRobots(u *url.URL) (*robots.Robots, error) {
	robotsLocation, err := robots.Locate(u.String())
	if err != nil {
		return nil, err
	}
	res, err := http.Get(robotsLocation)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	res.Body.Close()

	// robots.parseStart crashes if there are no tokens returned from lex()
	if len(bodyBytes) < 10 {
		return nil, errors.New("not enough bytes in robots.txt to consider parsing")
	}

	r, err := robots.From(res.StatusCode, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	return r, nil
}

func crawlAllowed(u *url.URL, ctx context.Context) error {
	crawlerLog := common.LoggerFromContext(ctx)

	// @FIX ignore robots.txt data past 500kb (https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt#file-format)
	// @TODO use noindex maybe? (https://developers.google.com/search/docs/crawling-indexing/block-indexing)

	if !common.Options.RespectRobots {
		return nil
	}

	robotsHost, ok := robotsMap[u.Host]
	if !ok {
		log.Fatal("robotsMap field un-initialized", "key", u.Host)
	}

	if robotsHost.robots == nil {
		log.Error(robotsHost)
		robotsHost.mutex.Lock()
		start := time.Now()
		r, err := fetchRobots(u)
		if err != nil {
			robotsHost.robots = &robots.Robots{}
		} else {
			robotsHost.robots = r
		}

		robotsMap[u.Host] = robotsHost
		robotsHost.mutex.Unlock()
		crawlerLog.Debug("fetched robots.txt", "duration", time.Since(start))
	}

	var skipRobots bool
	// extremely scuffed but.. fuck it we ball
	if fmt.Sprint(robotsHost.robots) == "&{false <nil>}" {
		skipRobots = true
	}

	if skipRobots || !robotsHost.robots.Test(common.Options.UserAgent, u.String()) {
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
				err := crawlAllowed(u, ctx)
				if err != nil {
					return err
				}

				var oldCrawlTime time.Time
				crawlDelayMap.Compute(
					u.Hostname(),
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

			fetchStart := time.Now()
			htm, err := parser.Fetch(urlString, ctx)
			if err != nil {
				return err
			}
			metrics.MeasureSince([]string{"fetch"}, fetchStart)
			kb := float32(len(htm)) / 1000

			parseStart := time.Now()
			urls, text, err := parser.ParsePage(htm, u, ctx)
			if err != nil {
				return err
			}

			urlsString := make([]string, len(urls))
			for i, u := range urls {
				urlsString[i] = u.String()
			}

			metrics.MeasureSince([]string{"parse"}, parseStart)

			// @TODO change .txt to match the mime of the page (support .pdfs, .jpg/.png, ect)
			writer(urlsString, fmt.Sprintf("%s/%s.txt", u.Hostname(), encodeUrl(urlString)), text)
			crawlerLog.Debug("crawled page", "urls", len(urls), "kb", kb, "duration", time.Since(fetchStart))

			return nil
		}(u)

		if err != nil {
			crawlerLog.Error("unable to crawl website", "error", err)
		}
	}
}

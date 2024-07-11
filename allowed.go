package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/temoto/robotstxt"
)

var robotsMap = xsync.NewMapOf[string, *robotstxt.Group]()

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

	logger := common.LoggerFromContext(ctx)

	robotsHost, _ := robotsMap.LoadOrCompute(u.Host, func() *robotstxt.Group {
		start := time.Now()
		g, err := fetchRobots(u, ctx)
		if err != nil {
			logger.Error("unable to fetch robots.txt", "error", err, "duration", time.Since(start))
			return &robotstxt.Group{}
		}

		logger.Debug("fetched robots.txt", "duration", time.Since(start))
		return g
	})

	if !robotsHost.Test(u.String()) {
		return errors.New("url was disalowed by robots")
	}

	return nil
}

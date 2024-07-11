package main

import (
	"context"
	"encoding/base64"
	"net/url"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/CelestialCrafter/crawler/pipeline"
	"github.com/charmbracelet/log"
)

func encodeUrl(urlString string) string {
	return base64.URLEncoding.EncodeToString([]byte(urlString))
}

type crawlData struct {
	ctx      context.Context
	cancel   context.CancelFunc
	url      *url.URL
	children []*url.URL
	original []byte
	text     []byte
}

func crawlPipeline(parser parsers.Parser, batch []*url.URL) (newUrls []*url.URL) {
	workers := common.Options.Workers
	queue := make([]*crawlData, len(batch))
	for i, u := range batch {
		logger := log.WithPrefix("crawler").With("url", u)
		queue[i] = &crawlData{
			url: u,
			ctx: context.WithValue(
				context.Background(),
				common.ContextLogger,
				logger,
			),
		}
	}

	input := pipeline.Gen(queue...)
	sleep := pipeline.Work(input, workers, func(data *crawlData) (*crawlData, error) {
		sleepTillCrawlable(data.url)
		return data, nil
	})

	allowed := pipeline.Work(sleep, workers, func(data *crawlData) (*crawlData, error) {
		data.ctx, data.cancel = context.WithTimeout(context.Background(), common.Options.CrawlTimeout)

		err := crawlAllowed(data.url, data.ctx)
		if err != nil {
			data.cancel()
			return nil, err
		}
		return data, nil
	})

	fetch := pipeline.Work(allowed, workers, func(data *crawlData) (*crawlData, error) {
		pageData, err := parser.Fetch(data.url.String(), data.ctx)
		data.cancel()

		if err != nil {
			return nil, err
		}

		data.original = pageData
		return data, nil
	})

	parse := pipeline.Work(fetch, workers, func(data *crawlData) (*crawlData, error) {
		links, text, err := parser.ParsePage(data.original, data.url)
		if err != nil {
			return nil, err
		}

		data.text = text
		data.children = links

		return data, nil
	})

	for result := range parse {
		if result.Err != nil {
			log.Warn("error pipelining", "error", result.Err)
			continue
		}

		item := *result.Item
		log.Info("pipeline result", "item", item.url.String())

		newUrls = append(newUrls, item.children...)
	}

	return
}

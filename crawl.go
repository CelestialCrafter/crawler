package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/charmbracelet/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/CelestialCrafter/crawler/parsers/basic"
	"github.com/CelestialCrafter/crawler/pipeline"
	pb "github.com/CelestialCrafter/crawler/protos"
)

func encodeUrl(urlString string) string {
	return base64.URLEncoding.EncodeToString([]byte(urlString))
}

type crawlData struct {
	ctx       context.Context
	cancel    context.CancelFunc
	url       *url.URL
	children  []*url.URL
	crawledAt time.Time
	mime      string
	original  []byte
	text      []byte
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

		data.crawledAt = time.Now()
		data.original = pageData
		return data, nil
	})

	parse := pipeline.Work(fetch, workers, func(data *crawlData) (*crawlData, error) {
		links, text, err := parser.ParsePage(data.original, data.url)
		if err != nil {
			return nil, err
		}

		m := string(bytes.Trim(data.original[:basic.MAX_MIME_BYTES], "\x00"))
		data.original = data.original[basic.MAX_MIME_BYTES:]

		data.mime = m
		data.text = text
		data.children = links

		return data, nil
	})

	write := pipeline.Work(parse, workers, func(data *crawlData) (*crawlData, error) {
		childrenString := make([]string, len(data.children))
		for i, u := range data.children {
			childrenString[i] = u.String()
		}

		crawled := &pb.Crawled{
			Url:       data.url.String(),
			Children:  childrenString,
			CrawledAt: timestamppb.New(data.crawledAt),
			Mime:      data.mime,
			Original:  data.original,
			Text:      data.text,
		}

		output, err := proto.Marshal(crawled)
		if err != nil {
			return nil, err
		}

		hostPath := path.Join(common.Options.CrawledPath, data.url.Host)

		_, err = os.Stat(hostPath)

		if err != nil {
			if os.IsNotExist(err) {
				err := os.Mkdir(hostPath, 0755)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}

		err = os.WriteFile(
			path.Join(hostPath, encodeUrl(data.url.Path)+".pb"),
			output,
			0644,
		)

		if err != nil {
			return nil, err
		}

		return data, nil
	})

	for result := range write {
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

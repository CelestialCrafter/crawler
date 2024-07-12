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
	"github.com/hashicorp/go-metrics"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/CelestialCrafter/crawler/parsers/basic"
	"github.com/CelestialCrafter/crawler/pipeline"
	pb "github.com/CelestialCrafter/crawler/protos"
)

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

func (c crawlData) String() string {
	return c.url.String()
}

func crawlPipeline(parser parsers.Parser, batch []*url.URL) (newUrls []*url.URL) {
	workers := common.Options.Workers
	metricsEnabled := true

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

	sleep := pipeline.Work(pipeline.WorkOptions[*crawlData, *crawlData]{
		Input:          input,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "sleep",
		Process: func(data *crawlData) (*crawlData, error) {
			sleepTillCrawlable(data.url)
			return data, nil
		},
	})

	allowed := pipeline.Work(pipeline.WorkOptions[*crawlData, *crawlData]{
		Input:          sleep,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "allowed",
		Process: func(data *crawlData) (*crawlData, error) {
			data.ctx, data.cancel = context.WithTimeout(context.Background(), common.Options.CrawlTimeout)

			err := crawlAllowed(data.url, data.ctx)
			if err != nil {
				data.cancel()
				return nil, err
			}
			return data, nil
		},
	})

	fetch := pipeline.Work(pipeline.WorkOptions[*crawlData, *crawlData]{
		Input:          allowed,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "fetch",
		Process: func(data *crawlData) (*crawlData, error) {
			pageData, err := parser.Fetch(data.url.String(), data.ctx)
			data.cancel()

			if err != nil {
				return nil, err
			}

			data.crawledAt = time.Now()
			data.original = pageData
			return data, nil
		},
	})

	parse := pipeline.Work(pipeline.WorkOptions[*crawlData, *crawlData]{
		Input:          fetch,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "parse",
		Process: func(data *crawlData) (*crawlData, error) {
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
		},
	})

	write := pipeline.Work(pipeline.WorkOptions[*crawlData, *crawlData]{
		Input:          parse,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "write",
		Process: func(data *crawlData) (*crawlData, error) {
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

			hostPath := path.Join(common.Options.DataPath, "crawled/", data.url.Host)

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

			if data.url.Path == "" {
				data.url.Path = "/"
			}

			err = os.WriteFile(
				path.Join(hostPath, base64.URLEncoding.EncodeToString([]byte(data.url.Path))+".pb"),
				output,
				0644,
			)

			if err != nil {
				return nil, err
			}

			return data, nil
		},
	})

	for result := range write {
		if result.Err != nil {
			log.Warn("error pipelining", "error", result.Err)
			continue
		}

		item := *result.Item
		log.Info("pipeline result", "item", item.url.String())
		metrics.IncrCounterWithLabels(
			[]string{"crawled_count"},
			1,
			[]metrics.Label{{
				Name:  "domain",
				Value: item.url.Hostname(),
			}},
		)

		newUrls = append(newUrls, item.children...)
	}

	return
}

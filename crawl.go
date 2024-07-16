package main

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-metrics"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers"
	"github.com/CelestialCrafter/crawler/pipeline"
	pb "github.com/CelestialCrafter/crawler/protos"
)

type crawlDataContext struct {
	ctx      context.Context
	url      *url.URL
	cancel   context.CancelFunc
	document pb.Document
}

func crawlPipeline(parser parsers.Parser, batch []string) (newUrls []string) {
	workers := common.Options.Workers
	metricsEnabled := true

	queue := make([]*crawlDataContext, len(batch))
	for i, urlString := range batch {
		logger := log.WithPrefix("crawler").With("url", urlString)
		u, err := url.Parse(urlString)
		if err != nil {
			log.Warn("error parsing url", "error", u)
			continue
		}

		queue[i] = &crawlDataContext{
			ctx: context.WithValue(
				context.Background(),
				common.ContextLogger,
				logger,
			),
			document: pb.Document{Url: urlString, Metadata: new(pb.Metadata)},
			url:      u,
		}
	}

	input := pipeline.Gen(queue...)

	sleep := pipeline.Work(pipeline.WorkOptions[*crawlDataContext, *crawlDataContext]{
		Input:          input,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "sleep",
		Process: func(data *crawlDataContext) (*crawlDataContext, error) {
			sleepTillCrawlable(data.url)
			return data, nil
		},
	})

	allowed := pipeline.Work(pipeline.WorkOptions[*crawlDataContext, *crawlDataContext]{
		Input:          sleep,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "allowed",
		Process: func(data *crawlDataContext) (*crawlDataContext, error) {
			data.ctx, data.cancel = context.WithTimeout(context.Background(), common.Options.CrawlTimeout)

			err := crawlAllowed(data.url, data.ctx)
			if err != nil {
				data.cancel()
				return nil, err
			}
			return data, nil
		},
	})

	fetch := pipeline.Work(pipeline.WorkOptions[*crawlDataContext, *crawlDataContext]{
		Input:          allowed,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "fetch",
		Process: func(data *crawlDataContext) (*crawlDataContext, error) {
			err := parser.Fetch(&data.document, data.ctx)
			data.cancel()

			if err != nil {
				return nil, err
			}

			data.document.Metadata.CrawledAt = timestamppb.New(time.Now())
			return data, nil
		},
	})

	parse := pipeline.Work(pipeline.WorkOptions[*crawlDataContext, *crawlDataContext]{
		Input:          fetch,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "parse",
		Process: func(data *crawlDataContext) (*crawlDataContext, error) {
			err := parser.ParsePage(&data.document, data.url)
			if err != nil {
				return nil, err
			}

			return data, nil
		},
	})

	write := pipeline.Work(pipeline.WorkOptions[*crawlDataContext, *crawlDataContext]{
		Input:          parse,
		Workers:        workers,
		MetricsEnabled: metricsEnabled,
		Name:           "write",
		Process: func(data *crawlDataContext) (*crawlDataContext, error) {
			output, err := proto.Marshal(&data.document)
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

			data.url.Path = strings.TrimPrefix(data.url.Path, "/")

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

		newUrls = append(newUrls, item.document.Children...)
	}

	return
}

package main

import (
	"context"
	"net/url"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-metrics"
	"github.com/valkey-io/valkey-go"

	"github.com/CelestialCrafter/crawler/common"
)

func loadNewBatch(vk valkey.Client, queue *[]*url.URL) error {
	*queue = make([]*url.URL, 0, common.Options.BatchSize)

	newBatchString, err := vk.Do(context.Background(),
		vk.
			B().
			Srandmember().
			Key("queue").
			Count(int64(common.Options.BatchSize)).
			Build(),
	).AsStrSlice()

	if err != nil {
		return err
	}

	for _, page := range newBatchString {
		u, err := url.Parse(page)
		if err != nil {
			log.Warn("could not parse url", "url", page)
			continue
		}

		*queue = append(*queue, u)
	}

	return nil
}

func cleanupBatch(vk valkey.Client, batch []*url.URL) error {
	ctx := context.Background()

	smoves := make(valkey.Commands, len(batch))
	for i, page := range batch {
		smoves[i] = vk.
			B().
			Smove().
			Source("queue").
			Destination("crawled").
			Member(page.String()).
			Build()
	}

	for _, resp := range vk.DoMulti(ctx, smoves...) {
		err := resp.Error()
		if err != nil {
			return err
		}
	}

	// @TODO use a domain label when you fix your metrics... stupid..
	metrics.IncrCounter([]string{"crawled_count"}, float32(common.Options.BatchSize))

	return nil
}

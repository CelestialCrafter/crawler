package main

import (
	"context"
	"net/url"

	"github.com/charmbracelet/log"
	"github.com/valkey-io/valkey-go"

	"github.com/CelestialCrafter/crawler/common"
)

func writeNewQueue(vk valkey.Client, newUrls *[]string) error {
	if len(*newUrls) < 1 {
		log.Warn("no new urls")
		return nil
	}

	vk.DoMulti(
		context.Background(),
		vk.
			B().
			Sadd().
			Key("queue").
			Member(*newUrls...).
			Build(),
		// filter out urls that have been crawled
		vk.
			B().
			Sdiffstore().
			Destination("queue").
			Key("queue").
			Key("crawled").
			Build(),
	)

	*newUrls = make([]string, 0)
	return nil
}

func loadNewBatch(vk valkey.Client) ([]*url.URL, error) {
	batch := make([]*url.URL, 0, common.Options.BatchSize)

	newBatchString, err := vk.Do(context.Background(),
		vk.
			B().
			Srandmember().
			Key("queue").
			Count(int64(common.Options.BatchSize)).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	for _, page := range newBatchString {
		u, err := url.Parse(page)
		if err != nil {
			log.Warn("unable to parse url", "url", page)
			continue
		}

		batch = append(batch, u)
	}

	return batch, nil
}

func cleanupBatch(vk valkey.Client, batch []*url.URL) error {
	ctx := context.Background()

	smoves := make(valkey.Commands, len(batch))
	for i, u := range batch {
		smoves[i] = vk.
			B().
			Smove().
			Source("queue").
			Destination("crawled").
			Member(u.String()).
			Build()
	}

	for _, resp := range vk.DoMulti(ctx, smoves...) {
		err := resp.Error()
		if err != nil {
			return err
		}
	}

	// @TODO use a domain label when you fix your metrics... stupid..

	return nil
}

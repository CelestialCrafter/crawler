package main

import (
	"context"
	"fmt"

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

func loadNewBatch(vk valkey.Client) ([]string, error) {
	batch, err := vk.Do(context.Background(),
		vk.
			B().
			Fcall().
			Function("QUEUESLICE").
			Numkeys(0).
			Arg(common.Options.QueuePrioritization).
			Arg(fmt.Sprint(common.Options.BatchSize*5)).
			Arg(fmt.Sprint(common.Options.BatchSize)).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	return batch, nil
}

func cleanupBatch(vk valkey.Client, batch []string) error {
	ctx := context.Background()

	smoves := make(valkey.Commands, len(batch))
	for i, urlString := range batch {
		smoves[i] = vk.
			B().
			Smove().
			Source("queue").
			Destination("crawled").
			Member(urlString).
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

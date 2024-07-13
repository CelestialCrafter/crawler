package main

import (
	"context"
	"os"
	"path"
	"time"

	"github.com/charmbracelet/log"
	pyroscope "github.com/grafana/pyroscope-go"

	"github.com/valkey-io/valkey-go"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers/basic"
)

func main() {
	_, err := common.LoadOptions()
	if err != nil {
		log.Fatal("unable to load options", "error", err)
	}

	// logging
	common.RecalculateLogOptions()
	logger := log.NewWithOptions(os.Stderr, common.LogOptions)
	log.SetDefault(logger)

	// profiling
	if common.Options.EnablePyroscope {
		_, err := pyroscope.Start(pyroscope.Config{
			ApplicationName: "crawler",
			ServerAddress:   common.Options.PyroscopeURI,
			Logger:          pyroscope.StandardLogger,
			UploadRate:      5 * time.Second,
		})
		if err != nil {
			log.Fatal("unable to start pyroscope", "error", err)
		}
	}

	startMetrics()

	// valkey
	vk, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{common.Options.ValkeyAddr},
	})
	if err != nil {
		log.Fatal("unable to connect to valkey", "error", err)
	}

	queueVkScript, err := os.ReadFile("valkey-queue.lua")
	if err != nil {
		log.Fatal("unable to read valkey queue script", "error", err)
	}

	err = vk.Do(
		context.Background(),
		vk.
			B().
			FunctionLoad().
			Replace().
			FunctionCode(string(queueVkScript)).
			Build(),
	).Error()

	if err != nil {
		log.Fatal("unable to load valkey queue script", "error", err)
	}

	defer vk.Close()

	// i/o init
	err = os.MkdirAll("data/", 0755)
	if err != nil {
		log.Fatal("unable to create data/ directory", "error", err)
	}

	err = populateInitialUrls(vk)
	if err != nil {
		log.Fatal("unable to populate database with initial urls", "error", err)
	}

	err = os.MkdirAll(path.Join(common.Options.DataPath, "crawled"), 0755)
	if err != nil {
		log.Fatal("unable to create crawled directory", "error", err)
	}

	// crawl loop
	parser := basic.New()

	var start time.Time
	for {
		start = time.Now()

		batch, err := loadNewBatch(vk)
		if err != nil {
			log.Fatal("unable to load new batches", "error", err)
		}

		if len(batch) < 1 {
			log.Warn("no new urls to be crawled; breaking.")
			break
		}

		batch = distributeQueue(batch)

		newUrlsUrl := crawlPipeline(parser, batch)
		newUrls := make([]string, len(newUrlsUrl))
		for i, u := range newUrlsUrl {
			newUrls[i] = u.String()
		}

		err = cleanupBatch(vk, batch)
		if err != nil {
			log.Fatal("unable to clean up batch", "error", err)
		}

		err = writeNewQueue(vk, &newUrls)
		if err != nil {
			log.Fatal("unable to write aggregated data", "error", err)
		}

		log.Info("batch finished", "duration", time.Since(start))
	}
}

package main

import (
	"context"
	"os"
	"runtime/pprof"
	"time"

	"github.com/charmbracelet/log"
	"github.com/grafana/pyroscope-go"

	"github.com/valkey-io/valkey-go"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/CelestialCrafter/crawler/parsers/basic"
)

func writeNewQueue(vk valkey.Client, newUrls *[]string) error {
	if len(*newUrls) < 1 {
		log.Warn("no new urls")
		return nil
	}

	vk.Do(context.Background(), vk.
		B().
		Sadd().
		Key("queue").Member(*newUrls...).Build())

	*newUrls = make([]string, 0)
	return nil
}

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
	if common.Options.EnablePprof {
		pf, err := os.Create(common.Options.PprofPath)
		if err != nil {
			log.Fatal("unable to open pprof file", "error", err)
			return
		}

		err = pprof.StartCPUProfile(pf)
		if err != nil {
			log.Fatal("unable to start cpu profile", "error", err)
		}
		defer pprof.StopCPUProfile()
	}

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

	err = os.MkdirAll(common.Options.CrawledPath, 0755)
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

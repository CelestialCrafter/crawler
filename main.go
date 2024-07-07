package main

import (
	"archive/tar"
	"context"
	"net/url"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/charmbracelet/log"

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
	if common.Options.EnableProfiler {
		pf, err := os.Create("data/crawler.prof")
		if err != nil {
			log.Fatal("could not open profiler file", "error", err)
			return
		}

		err = pprof.StartCPUProfile(pf)
		if err != nil {
			log.Fatal("could not start cpu profile", "error", err)
		}
		defer pprof.StopCPUProfile()
	}

	startMetrics()

	// valkey
	vk, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{"127.0.0.1:6379"},
	})
	if err != nil {
		log.Fatal("could not connect to valkey", "error", err)
	}

	defer vk.Close()

	// i/o init
	err = os.MkdirAll("data/", 0644)
	if err != nil {
		log.Fatal("unable to create data/ directory", "error", err)
	}

	err = populateInitialUrls(vk)
	if err != nil {
		log.Fatal("unable to populate database with initial urls", "error", err)
	}

	cf, err := os.Create(common.Options.CrawledTarPath)
	if err != nil {
		log.Fatal("unable to open crawled tarball", "error", err)
	}
	defer cf.Close()

	cw := tar.NewWriter(cf)
	defer cw.Close()

	// crawl loop
	parser := basic.New()
	var queue []*url.URL
	var queueCopy []*url.URL
	newUrls := make([]string, 0)

	var start time.Time
	for {
		start = time.Now()
		var wg sync.WaitGroup

		err = loadNewBatch(vk, &queue)
		if err != nil {
			log.Fatal("could not load new batches", "error", err)
		}

		queueCopy = queue

		if len(queue) < 1 {
			log.Warn("no new urls to be crawled; breaking.")
			break
		}

		preflight(queue)
		for i := range common.Options.Workers {
			wg.Add(1)

			go func(i int) {
				defer func() {
					wg.Done()
				}()

				worker(i, &queue, parser, func(urls []string, s string, b []byte) {
					newUrls = append(newUrls, urls...)
					err := writeTar(cw, s, b)
					if err != nil {
						log.Error("could not write to tar", "error", err)
					}
				})
			}(i)
		}

		wg.Wait()
		err := cleanupBatch(vk, queueCopy)
		if err != nil {
			log.Fatal("could not clean up batch", "error", err)
		}

		err = writeNewQueue(vk, &newUrls)
		if err != nil {
			log.Fatal("could not write aggregated data", "error", err)
		}

		log.Info("batch finished", "duration", time.Since(start))
	}
}

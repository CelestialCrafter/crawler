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
			log.Fatal("unable to load new batches", "error", err)
		}

		queueCopy = queue

		if len(queue) < 1 {
			log.Warn("no new urls to be crawled; breaking.")
			break
		}

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
						log.Error("unable to write to tar", "error", err)
					}
				})
			}(i)
		}

		wg.Wait()
		err := cleanupBatch(vk, queueCopy)
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

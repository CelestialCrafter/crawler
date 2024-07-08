package main

import (
	"archive/tar"
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-metrics"
	prometheus "github.com/hashicorp/go-metrics/prometheus"
	"github.com/valkey-io/valkey-go"

	"github.com/CelestialCrafter/crawler/common"
)

func writeTar(tw *tar.Writer, name string, data []byte) error {
	header := new(tar.Header)
	header.Name = name
	header.Size = int64(len(data))
	header.Mode = 0644
	header.ModTime = time.Now()

	err := tw.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = tw.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func startMetrics() {
	var crawlerSink metrics.MetricSink
	var err error

	if common.Options.EnableMetrics {
		crawlerSink, err = prometheus.NewPrometheusPushSink(common.Options.PrometheusPushAddr, 5*time.Second, "crawler_sink")
		if err != nil {
			log.Fatal("unable to create prometheus sink (push)", "error", err)
		}
	} else {
		crawlerSink = &metrics.BlackholeSink{}
	}

	config := metrics.DefaultConfig("crawler")
	config.EnableRuntimeMetrics = false
	_, err = metrics.NewGlobal(config, crawlerSink)
	if err != nil {
		log.Fatal("unable to create crawler metrics", "error", err)
	}

}

func populateInitialUrls(vk valkey.Client) error {
	if len(common.Options.InitialPages) < 1 {
		log.Warn("no urls in initial urls")
		return nil
	}

	ctx := context.Background()
	queueLength, err := vk.Do(ctx, vk.B().Scard().Key("queue").Build()).AsInt64()
	if err != nil {
		return err
	}

	if queueLength > 0 && common.Options.Recover {
		return nil
	}

	err = vk.Do(
		context.Background(),
		vk.
			B().
			Sadd().
			Key("queue").
			Member(common.Options.InitialPages...).
			Build(),
	).Error()

	if err != nil {
		return err
	}

	return nil
}

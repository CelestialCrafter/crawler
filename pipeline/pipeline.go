package pipeline

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/go-metrics"
)

type Result[I any] struct {
	Err  error
	Item *I
}

type WorkOptions[I any, O any] struct {
	Input          <-chan Result[I]
	Workers        int
	Process        func(I) (O, error)
	Name           string
	MetricsEnabled bool
}

func Gen[I any](inputs ...I) <-chan Result[I] {
	output := make(chan Result[I], len(inputs))
	for _, item := range inputs {
		output <- Result[I]{
			Item: &item,
			Err:  nil,
		}
	}
	close(output)
	return output
}

func logMetrics(worker int, start time.Time, name string, metricsEnabled bool) {
	if metricsEnabled {
		metrics.MeasureSinceWithLabels(
			[]string{"pipeline_step"},
			start,
			[]metrics.Label{
				{
					Name:  "worker",
					Value: fmt.Sprint(worker),
				},
				{
					Name:  "name",
					Value: name,
				},
			},
		)
	}
}

func Work[I any, O any](opts WorkOptions[I, O]) <-chan Result[O] {
	logger := log.With("name", opts.Name)
	output := make(chan Result[O], opts.Workers)
	var wg sync.WaitGroup

	process := func(worker int, item Result[I]) {
		logger.With("worker", worker)
		start := time.Now()
		if item.Err != nil {
			output <- Result[O]{
				Err:  item.Err,
				Item: nil,
			}
			return
		}

		raw, err := opts.Process(*item.Item)
		if err != nil {
			logger.Debug("unable to process item", "error", err)
		} else {
			logger.Debug("processed item", "item", raw, "worker", worker)
			logMetrics(worker, start, opts.Name, opts.MetricsEnabled)

		}

		output <- Result[O]{
			Err:  err,
			Item: &raw,
		}
	}
	worker := func(worker int) {
		defer wg.Done()

		for item := range opts.Input {
			process(worker, item)
		}
		log.Debug("worker exiting", "name", opts.Name, "worker", worker)
	}

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go worker(i)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}

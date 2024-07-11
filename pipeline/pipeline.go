package pipeline

import (
	"github.com/charmbracelet/log"
	"github.com/puzpuzpuz/xsync/v3"
)

type Result[I any] struct {
	Err  error
	Item *I
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

func Work[I any, O any](
	input <-chan Result[I],
	workers int,
	process func(I) (O, error),
) <-chan Result[O] {
	output := make(chan Result[O], workers)
	exits := xsync.NewCounter()

	worker := func(worker int) {
		for item := range input {
			if item.Err != nil {
				output <- Result[O]{
					Err:  item.Err,
					Item: nil,
				}
				continue
			}

			raw, err := process(*item.Item)
			if err != nil {
				log.Debug("unable to process item", "error", err)
			} else {
				log.Debug("processed item", "item", raw, "worker", worker)
			}

			output <- Result[O]{
				Err:  err,
				Item: &raw,
			}
		}
		exits.Add(1)

		if exits.Value() == int64(workers) {
			close(output)
		}
	}

	for i := 0; i < workers; i++ {
		go worker(i)
	}

	return output
}

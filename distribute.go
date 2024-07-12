package main

import (
	"net/url"

	"github.com/charmbracelet/log"
)

// ai generated, edited by me - claude 3.5 sonnet
func distributeQueue(urls []*url.URL) []*url.URL {
	// Count occurrences of each hostname
	hostnameCounts := make(map[string]int)
	for _, u := range urls {
		hostnameCounts[u.Hostname()]++
	}

	// Calculate the size of the output array
	outputSize := len(urls)

	// Initialize the output array and position trackers
	output := make([]*url.URL, outputSize)
	positions := make(map[string][]int)

	// Calculate ideal positions for each hostname
	for hostname, count := range hostnameCounts {
		step := float64(outputSize) / float64(count)
		positions[hostname] = make([]int, count)
		for i := 0; i < count; i++ {
			positions[hostname][i] = int(float64(i) * step)
		}
	}

	// Place URLs in their positions
	for _, u := range urls {
		hostname := u.Hostname()
		pos := positions[hostname][0]
		positions[hostname] = positions[hostname][1:] // Remove the used position

		for output[pos] != nil {
			pos = (pos + 1) % outputSize
		}
		output[pos] = u
	}

	log.Fatal(output)
	return output
}

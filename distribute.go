package main

import "net/url"

// ai generated, edited by me - claude 3.5 sonnet
func distributeQueue(urls []*url.URL) []*url.URL {
	// Count occurrences of each host
	hostCounts := make(map[string]int)
	for _, u := range urls {
		hostCounts[u.Host]++
	}

	// Calculate the size of the output array
	outputSize := len(urls)

	// Initialize the output array and position trackers
	output := make([]*url.URL, outputSize)
	positions := make(map[string][]int)

	// Calculate ideal positions for each host
	for host, count := range hostCounts {
		step := float64(outputSize) / float64(count)
		positions[host] = make([]int, count)
		for i := 0; i < count; i++ {
			positions[host][i] = int(float64(i) * step)
		}
	}

	// Place URLs in their positions
	for _, u := range urls {
		host := u.Host
		pos := positions[host][0]
		positions[host] = positions[host][1:] // Remove the used position

		for output[pos] != nil {
			pos = (pos + 1) % outputSize
		}
		output[pos] = u
	}

	return output
}

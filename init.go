package main

import (
	"archive/tar"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/charmbracelet/log"
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

func mapify[T comparable](slice []T) map[T]struct{} {
	mapped := map[T]struct{}{}
	for _, e := range slice {
		mapped[e] = struct{}{}
	}

	return mapped
}

func chooseStartUrls() (map[*url.URL]struct{}, error) {
	var err error
	if common.Options.Recover {
		_, err = os.Stat(common.Options.CrawledListPath)
	} else {
		err = os.ErrNotExist
	}

	if errors.Is(err, os.ErrNotExist) {
		log.Info("using initial urls")
		urlsSlice, err := stringsToUrls(common.Options.InitialPages)
		if err != nil {
			return nil, err
		}
		return mapify(urlsSlice), nil
	}

	if err != nil {
		return nil, err
	}

	log.Info("using frontier urls")
	urlsBytes, err := os.ReadFile(common.Options.FrontierPath)
	if err != nil {
		log.Fatal("unable to read initial sites", "error", err)
	}

	urlsStrings := strings.Split(string(urlsBytes), "\n")

	frontier, err := stringsToUrls(urlsStrings)
	if err != nil {
		return nil, err
	}
	return mapify(frontier), nil
}

func initializeCrawledList() (map[*url.URL]struct{}, error) {
	crawledList := map[*url.URL]struct{}{}

	var err error
	if common.Options.Recover {
		_, err = os.Stat(common.Options.CrawledListPath)
	} else {
		err = os.ErrNotExist
	}

	if err == nil {
		crawledListBytes, err := os.ReadFile(common.Options.FrontierPath)
		if err != nil {
			return nil, err
		}

		for _, urlString := range strings.Split(string(crawledListBytes), "\n") {
			url, err := url.Parse(urlString)
			if err != nil {
				log.Warn("unable to parse url", "error", err, "url", url)
				continue
			}
			crawledList[url] = struct{}{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return crawledList, nil
}

package common

import (
	"time"

	"github.com/go-ini/ini"
)

type OptionsStructure struct {
	CrawledListPath string
	FrontierPath    string
	CrawledTarPath  string

	MaxConcurrentCrawlers int
	CrawlDepth            int
	Recover               bool
	RequestTimeout        time.Duration

	InitialPages []string
}

const OPTIONS_PATH = "options.ini"

var Options OptionsStructure

func LoadOptions() (OptionsStructure, error) {
	optionsIni, err := ini.Load(OPTIONS_PATH)
	if err != nil {
		return OptionsStructure{}, err
	}

	pathsSection := optionsIni.Section("paths")
	settingsSection := optionsIni.Section("settings")
	blankSection := optionsIni.Section("")

	Options = OptionsStructure{
		CrawledListPath: pathsSection.Key("crawled_list_path").MustString("data/crawled-list"),
		FrontierPath:    pathsSection.Key("frontier_path").MustString("data/frontier"),
		CrawledTarPath:  pathsSection.Key("crawled_tar_path").MustString("data/crawled.tar"),
		InitialPages:    blankSection.Key("initial").Strings(","),

		MaxConcurrentCrawlers: settingsSection.Key("max_concurrent_crawlers").MustInt(20),
		CrawlDepth:            settingsSection.Key("crawl_depth").MustInt(0),
		Recover:               settingsSection.Key("recover").MustBool(true),
		RequestTimeout:        settingsSection.Key("request_timeout").MustDuration(10 * time.Second),
	}

	return Options, nil
}

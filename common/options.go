package common

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-ini/ini"
)

type OptionsStructure struct {
	CrawledListPath string
	FrontierPath    string
	CrawledTarPath  string

	LogLevel              log.Level
	UserAgent             string
	MaxConcurrentCrawlers int
	CrawlDepth            int
	Recover               bool
	CrawlTimeout          time.Duration
	DefaultCrawlDelay     time.Duration
	RespectRobots         bool

	EnableProfiler bool
	ProfilerPath   string
	EnableMetrics  bool
	StatsdURI      string

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
	performanceSection := optionsIni.Section("performance")
	blankSection := optionsIni.Section("")

	logLevel, err := log.ParseLevel(settingsSection.Key("log_level").MustString("info"))
	if err != nil {
		log.Fatal("unable to parse log level option")
	}

	Options = OptionsStructure{
		CrawledListPath: pathsSection.Key("crawled_list_path").MustString("data/crawled-list"),
		FrontierPath:    pathsSection.Key("frontier_path").MustString("data/frontier"),
		CrawledTarPath:  pathsSection.Key("crawled_tar_path").MustString("data/crawled.tar"),

		LogLevel:              logLevel,
		UserAgent:             settingsSection.Key("user_agent").MustString("Mozilla/5.0 (compatible; Crawler/1.0; +http://www.google.com/bot.html)"),
		MaxConcurrentCrawlers: settingsSection.Key("max_concurrent_crawlers").MustInt(20),
		CrawlDepth:            settingsSection.Key("crawl_depth").MustInt(0),
		Recover:               settingsSection.Key("recover").MustBool(true),
		CrawlTimeout:          settingsSection.Key("crawl_timeout").MustDuration(10 * time.Second),
		DefaultCrawlDelay:     settingsSection.Key("default_crawl_delay").MustDuration(500 * time.Millisecond),
		RespectRobots:         settingsSection.Key("respect_robots").MustBool(true),

		EnableProfiler: settingsSection.Key("enable_profiler").MustBool(false),
		ProfilerPath:   performanceSection.Key("profiler_path").MustString("data/crawler.prof"),
		EnableMetrics:  settingsSection.Key("enable_metrics").MustBool(false),
		StatsdURI:      performanceSection.Key("statsd_uri").MustString("localhost:8125"),

		InitialPages: blankSection.Key("initial").Strings(","),
	}

	return Options, nil
}

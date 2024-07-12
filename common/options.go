package common

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-ini/ini"
)

type OptionsStructure struct {
	ValkeyAddr string
	DataPath   string

	LogLevel            log.Level
	UserAgent           string
	QueuePrioritization string
	Workers             int
	BatchSize           int
	Recover             bool
	CrawlTimeout        time.Duration
	DefaultCrawlDelay   time.Duration
	RespectRobots       bool

	EnablePprof bool
	PprofPath   string

	EnablePyroscope bool
	PyroscopeURI    string

	EnableMetrics      bool
	PrometheusPushAddr string

	InitialPages []string
}

const OPTIONS_PATH = "options.ini"

var Options OptionsStructure

func LoadOptions() (OptionsStructure, error) {
	optionsIni, err := ini.Load(OPTIONS_PATH)
	if err != nil {
		return OptionsStructure{}, err
	}

	settingsSection := optionsIni.Section("settings")
	performanceSection := optionsIni.Section("performance")
	blankSection := optionsIni.Section("")

	logLevel, err := log.ParseLevel(settingsSection.Key("log_level").MustString("info"))
	if err != nil {
		log.Fatal("unable to parse log level option")
	}

	Options = OptionsStructure{
		ValkeyAddr: blankSection.Key("valkey_addr").MustString("localhost:6379"),
		DataPath:   blankSection.Key("data_path").MustString("data/"),

		LogLevel:            logLevel,
		QueuePrioritization: settingsSection.Key("queue_prioritization").MustString("mean"),
		UserAgent:           settingsSection.Key("user_agent").MustString("Mozilla/5.0 (compatible; Crawler/1.0; +http://www.google.com/bot.html)"),
		Workers:             settingsSection.Key("workers").MustInt(50),
		BatchSize:           settingsSection.Key("batch_size").MustInt(100),
		Recover:             settingsSection.Key("recover").MustBool(true),
		CrawlTimeout:        settingsSection.Key("crawl_timeout").MustDuration(5 * time.Second),
		DefaultCrawlDelay:   settingsSection.Key("default_crawl_delay").MustDuration(500 * time.Millisecond),
		RespectRobots:       settingsSection.Key("respect_robots").MustBool(true),

		EnablePprof:     performanceSection.Key("enable_pprof").MustBool(false),
		EnablePyroscope: performanceSection.Key("enable_pyroscope").MustBool(false),
		PyroscopeURI:    performanceSection.Key("pyroscope_uri").MustString("http://localhost:4040"),

		EnableMetrics:      performanceSection.Key("enable_metrics").MustBool(false),
		PrometheusPushAddr: performanceSection.Key("prometheus_push_addr").MustString(":9091"),

		// @NOTE length of initial pages MUST be under batch size * workers
		InitialPages: blankSection.Key("initial").Strings(","),
	}

	return Options, nil
}

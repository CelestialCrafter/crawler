package common

import (
	"reflect"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

type logLevel struct {
	log.Level
}

func (l *logLevel) UnmarshalText(text []byte) error {
	var err error
	l.Level, err = log.ParseLevel(string(text))
	return err
}

type OptionsStructure struct {
	InitialPages        []string      `toml:"initial_pages"`
	DataPath            string        `toml:"data_path"`
	LogLevel            logLevel      `toml:"log_level"`
	UserAgent           string        `toml:"user_agent"`
	QueuePrioritization string        `toml:"queue_prioritization"`
	Workers             int           `toml:"workers"`
	BatchSize           int           `toml:"batch_size"`
	Recover             bool          `toml:"recover"`
	CrawlTimeout        time.Duration `toml:"crawl_timeout"`
	DefaultCrawlDelay   time.Duration `toml:"default_crawl_delay"`
	RespectRobots       bool          `toml:"respect_robots"`

	ValkeyAddr string `toml:"services_valkey_addr"`

	EnablePyroscope bool   `toml:"services_enable_pyroscope"`
	PyroscopeURI    string `toml:"services_pyroscope_uri"`

	EnableMetrics      bool   `toml:"services_enable_metrics"`
	PrometheusPushAddr string `toml:"services_prometheus_push_addr"`
}

var Options OptionsStructure
var Default = OptionsStructure{
	InitialPages:        []string{"https://arxiv.org"},
	DataPath:            "data/",
	LogLevel:            logLevel{Level: log.InfoLevel},
	QueuePrioritization: "mean",
	UserAgent:           "Mozilla/5.0 (compatible; Crawler/1.0; +http://www.google.com/bot.html)",
	Workers:             50,
	BatchSize:           100,
	Recover:             true,
	CrawlTimeout:        5 * time.Second,
	DefaultCrawlDelay:   500 * time.Millisecond,
	RespectRobots:       true,

	ValkeyAddr: "localhost:6379",

	EnablePyroscope: false,
	PyroscopeURI:    "http://localhost:4040",

	EnableMetrics:      false,
	PrometheusPushAddr: ":9091",
}

const OPTIONS_PATH = "options.toml"

func LoadOptions() (OptionsStructure, error) {
	_, err := toml.DecodeFile(OPTIONS_PATH, &Options)
	if err != nil {
		return OptionsStructure{}, err
	}

	// set default values for keys not found in options file
	t := reflect.ValueOf(&Options).Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsZero() {
			continue
		}

		f.Set(reflect.ValueOf(Default).Field(i))
	}

	return Options, nil
}

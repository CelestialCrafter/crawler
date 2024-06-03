package common

import "github.com/charmbracelet/log"

var LogOptions = log.Options{
	ReportTimestamp: true,
	ReportCaller:    true,
	Level:           Options.LogLevel,
}

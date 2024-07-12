package main

import (
	"net/url"
	"time"

	"github.com/CelestialCrafter/crawler/common"
	"github.com/puzpuzpuz/xsync/v3"
)

var crawlDelayMap = xsync.NewMapOf[string, time.Time]()

func sleepTillCrawlable(u *url.URL) {
	if common.Options.DefaultCrawlDelay == time.Second*0 {
		return
	}

	var oldCrawlTime time.Time
	crawlDelayMap.Compute(
		u.Host,
		func(oldValue time.Time, loaded bool) (newValue time.Time, delete bool) {
			delete = false
			oldCrawlTime = oldValue

			var startingPoint time.Time
			now := time.Now()

			if oldValue.After(now) {
				startingPoint = oldValue
			} else {
				startingPoint = now
			}

			newValue = startingPoint.Add(common.Options.DefaultCrawlDelay)

			return
		},
	)

	if time.Now().Before(oldCrawlTime) {
		time.Sleep(time.Until(oldCrawlTime))
	}
}

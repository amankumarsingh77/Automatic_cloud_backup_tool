package utils

import (
	"github.com/robfig/cron/v3"
	"time"
)

func CronToDuration(cronstr string) (time.Duration, error) {
	spec, err := cron.ParseStandard(cronstr)
	if err != nil {
		return 0, err
	}
	nextTime := spec.Next(time.Now())
	return nextTime.Sub(time.Now()), nil
}

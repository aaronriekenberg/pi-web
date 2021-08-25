package utils

import "time"

const (
	CacheControlHeaderKey      = "cache-control"
	MaxAgeZero                 = "max-age=0"
	ContentTypeHeaderKey       = "content-type"
	ContentTypeTextHTML        = "text/html"
	ContentTypeTextPlain       = "text/plain"
	ContentTypeApplicationJSON = "application/json"
)

const timeFormat = "Mon Jan 2 15:04:05.000000000 -0700 MST 2006"

func FormatTime(t time.Time) string {
	return t.Format(timeFormat)
}

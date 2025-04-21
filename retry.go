package gotsk

import "time"

func simpleBackoff(attempt int) time.Duration {
	return time.Second * time.Duration(attempt+1)
}

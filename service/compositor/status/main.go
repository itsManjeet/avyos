package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	TimeFormat = "Mon, Jan 02  03:04 pm"
)

func main() {
	for {
		var sb strings.Builder
		now := time.Now()
		sb.WriteString(now.Format(TimeFormat))
		fmt.Printf(" %s \n", sb.String())
		time.Sleep(time.Second)
	}

}

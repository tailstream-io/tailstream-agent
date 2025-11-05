package main

import (
	"fmt"
	"net/http"
	"runtime"
)

func setDefaultUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", fmt.Sprintf("tailstream-agent/%s (%s; %s)", Version, runtime.GOOS, runtime.GOARCH))
}

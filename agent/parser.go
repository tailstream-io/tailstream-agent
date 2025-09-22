package main

import (
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strconv"
)

// Event is the normalized record to send to Tailstream.
type Event map[string]interface{}

// AccessLogEntry represents a parsed access log entry
type AccessLogEntry struct {
	Host      string  `json:"host"`
	Path      string  `json:"path"`
	Method    string  `json:"method"`
	Status    int     `json:"status"`
	RT        float64 `json:"rt"`
	Bytes     int64   `json:"bytes"`
	Src       string  `json:"src"`
	IP        string  `json:"ip,omitempty"`
	UserAgent string  `json:"user_agent,omitempty"`
	TS        int64   `json:"ts,omitempty"`
}

// Common access log format patterns
var (
	// Apache/Nginx Common Log Format: IP - - [timestamp] "METHOD path HTTP/version" status bytes
	commonLogRegex = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) ([^"]*) HTTP/[^"]*" (\d+) (\S+)`)

	// Apache/Nginx Combined Log Format (includes referer and user-agent)
	combinedLogRegex = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) ([^"]*) HTTP/[^"]*" (\d+) (\S+) "([^"]*)" "([^"]*)"`)

	// Nginx with response time: IP - - [timestamp] "METHOD path HTTP/version" status bytes "referer" "user-agent" rt
	nginxWithRTRegex = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) ([^"]*) HTTP/[^"]*" (\d+) (\S+) "([^"]*)" "([^"]*)" ([0-9.]+)`)
)

// parseAccessLog attempts to parse common access log formats
func parseAccessLog(line, filename, hostname string) (*AccessLogEntry, bool) {
	// Try nginx with response time first (most detailed)
	if matches := nginxWithRTRegex.FindStringSubmatch(line); matches != nil {
		status, _ := strconv.Atoi(matches[5])
		bytes, _ := strconv.ParseInt(matches[6], 10, 64)
		rt, _ := strconv.ParseFloat(matches[9], 64)

		return &AccessLogEntry{
			Host:      hostname,
			Path:      matches[4],
			Method:    matches[3],
			Status:    status,
			RT:        rt,
			Bytes:     bytes,
			Src:       filename,
			IP:        matches[1],
			UserAgent: matches[8],
		}, true
	}

	// Try combined log format
	if matches := combinedLogRegex.FindStringSubmatch(line); matches != nil {
		status, _ := strconv.Atoi(matches[5])
		bytes, _ := strconv.ParseInt(matches[6], 10, 64)

		return &AccessLogEntry{
			Host:      hostname,
			Path:      matches[4],
			Method:    matches[3],
			Status:    status,
			RT:        0.0, // No response time in combined format
			Bytes:     bytes,
			Src:       filename,
			IP:        matches[1],
			UserAgent: matches[8],
		}, true
	}

	// Try common log format
	if matches := commonLogRegex.FindStringSubmatch(line); matches != nil {
		status, _ := strconv.Atoi(matches[5])
		bytes, _ := strconv.ParseInt(matches[6], 10, 64)

		return &AccessLogEntry{
			Host:   hostname,
			Path:   matches[4],
			Method: matches[3],
			Status: status,
			RT:     0.0, // No response time in common format
			Bytes:  bytes,
			Src:    filename,
			IP:     matches[1],
		}, true
	}

	return nil, false
}

// parseLine normalizes a log line into the required Tailstream format
func parseLine(ll LogLine, host string) (Event, bool) {
	// Try to parse as JSON first
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ll.Line), &m); err == nil {
		// Ensure required fields are present
		if _, hasHost := m["host"]; !hasHost {
			m["host"] = host
		}
		if _, hasSrc := m["src"]; !hasSrc {
			m["src"] = ll.File
		}
		return m, true
	}

	// Try to parse as access log
	if entry, ok := parseAccessLog(ll.Line, ll.File, host); ok {
		// Convert to Event map
		data, _ := json.Marshal(entry)
		var event Event
		json.Unmarshal(data, &event)
		return event, true
	}

	// Skip unparseable lines
	if os.Getenv("DEBUG") == "1" {
		log.Printf("Skipping unparseable line from %s: %s", ll.File, ll.Line)
	}
	return nil, false
}
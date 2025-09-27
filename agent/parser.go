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
			Src:       matches[1],
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
			Src:       matches[1],
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
			Src:    matches[1],
		}, true
	}

	return nil, false
}

// parseCustomFormat parses a log line using a custom format definition
func parseCustomFormat(line, filename, hostname string, format *LogFormat) (Event, bool) {
	if format == nil || format.Pattern == "" {
		return nil, false
	}

	regex, err := regexp.Compile(format.Pattern)
	if err != nil {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("Invalid regex pattern in custom format %s: %v", format.Name, err)
		}
		return nil, false
	}

	matches := regex.FindStringSubmatch(line)
	if matches == nil {
		return nil, false
	}

	event := make(Event)

	// Apply default values first
	for key, value := range format.Default {
		event[key] = value
	}

	// Map regex groups to fields
	for field, groupName := range format.Fields {
		if groupName == "hostname" {
			event[field] = hostname
		} else if groupName == "filename" {
			event[field] = filename
		} else {
			// Try to parse as group number first
			if groupNum, err := strconv.Atoi(groupName); err == nil && groupNum > 0 && groupNum < len(matches) {
				value := matches[groupNum]

				// Try to convert to appropriate type based on field name
				switch field {
				case "status":
					if intVal, err := strconv.Atoi(value); err == nil {
						event[field] = intVal
					} else {
						event[field] = value
					}
				case "bytes":
					if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
						event[field] = intVal
					} else {
						event[field] = value
					}
				case "rt":
					if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
						event[field] = floatVal
					} else {
						event[field] = value
					}
				default:
					event[field] = value
				}
			}
		}
	}

	// Ensure required fields have defaults if not set
	if _, hasHost := event["host"]; !hasHost {
		event["host"] = hostname
	}
	// Only set src to filename if it wasn't explicitly mapped to something else in the custom format
	if _, hasSrc := event["src"]; !hasSrc {
		// Check if src was explicitly mapped to "filename" in the format
		if srcMapping, ok := format.Fields["src"]; ok && srcMapping == "filename" {
			event["src"] = filename
		}
		// Otherwise, don't set src automatically - let it be determined by parsing logic
	}

	return event, true
}

// parseLine normalizes a log line into the required Tailstream format
func parseLine(ll LogLine, host string, customFormat *LogFormat) (Event, bool) {
	// Try custom format first if provided
	if customFormat != nil {
		if event, ok := parseCustomFormat(ll.Line, ll.File, host, customFormat); ok {
			return event, true
		}
	}

	// Try to parse as JSON first
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ll.Line), &m); err == nil {
		// Ensure required fields are present
		if _, hasHost := m["host"]; !hasHost {
			m["host"] = host
		}
		// Don't set src to filename for JSON logs - let the JSON data determine it
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
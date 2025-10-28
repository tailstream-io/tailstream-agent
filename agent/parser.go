package main

// Event is the normalized record to send to Tailstream.
// Events are sent as structured JSON with metadata - the backend handles log parsing.
type Event interface{}

// LogEvent represents a log line with metadata about its source
type LogEvent struct {
	Log      string `json:"log"`
	Filename string `json:"filename"`
}

// parseLine returns the log line with filename metadata - backend handles all parsing
func parseLine(ll LogLine) (Event, bool) {
	// Send the raw line with filename metadata
	// The backend will parse the log field while having context about its source
	return LogEvent{
		Log:      ll.Line,
		Filename: ll.File,
	}, true
}

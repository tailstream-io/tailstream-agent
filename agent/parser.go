package main

// Event is the normalized record to send to Tailstream.
// Events are sent as raw strings - the backend handles all parsing.
type Event interface{}

// parseLine returns the log line as a raw string - backend handles all parsing
func parseLine(ll LogLine) (Event, bool) {
	// Send the raw line as-is - backend will parse it
	// This keeps the agent simple and lets the backend handle format detection
	return ll.Line, true
}

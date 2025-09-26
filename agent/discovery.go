package main

import (
	"github.com/bmatcuk/doublestar/v4"
)

// StreamFileMapping maps stream configurations to their matching files
type StreamFileMapping struct {
	Stream StreamConfig
	Files  []string
}

// discover finds log files and maps them to appropriate streams
func discover(cfg Config) ([]StreamFileMapping, error) {
	var mappings []StreamFileMapping

	for _, stream := range cfg.Streams {
		var files []string
		for _, pattern := range stream.Paths {
			matches, err := doublestar.FilepathGlob(pattern)
			if err != nil {
				continue
			}
			for _, m := range matches {
				if excluded(m, stream.Exclude) {
					continue
				}
				files = append(files, m)
			}
		}
		if len(files) > 0 {
			mappings = append(mappings, StreamFileMapping{
				Stream: stream,
				Files:  files,
			})
		}
	}

	return mappings, nil
}

func excluded(path string, patterns []string) bool {
	for _, p := range patterns {
		ok, err := doublestar.Match(p, path)
		if err == nil && ok {
			return true
		}
	}
	return false
}
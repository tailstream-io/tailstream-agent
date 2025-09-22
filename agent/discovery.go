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

	// If using multi-stream config
	if len(cfg.Streams) > 0 {
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

	// Legacy single-stream discovery (backward compatibility)
	var files []string
	for _, pattern := range cfg.Discovery.Paths.Include {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if excluded(m, cfg.Discovery.Paths.Exclude) {
				continue
			}
			files = append(files, m)
		}
	}

	if len(files) > 0 {
		// Create a default stream from legacy config
		legacyStream := StreamConfig{
			Name:     "default",
			StreamID: cfg.Ship.StreamID,
			Key:      cfg.Key,
			Paths:    cfg.Discovery.Paths.Include,
			Exclude:  cfg.Discovery.Paths.Exclude,
		}

		// For legacy config, use the full URL if provided
		if cfg.Ship.URL != "" {
			legacyStream.LegacyURL = cfg.Ship.URL
		}

		mappings = append(mappings, StreamFileMapping{
			Stream: legacyStream,
			Files:  files,
		})
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
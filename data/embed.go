// Package data embeds the game content (see CONTENT.md) so the Go build ships as a single executable.
package data

import "embed"

//go:embed *.json sprites/*.json sprites/*.png sprites/*.gif types/*.png ui/* art/*
var FS embed.FS

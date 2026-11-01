package web

import "embed"

// FS contains the embedded browser surfaces. The embed directive lives here
// because Go embed cannot reference a parent directory.
//
//go:embed control/* overlay/*
var FS embed.FS

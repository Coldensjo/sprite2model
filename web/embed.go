// Package web holds the converter's page: served by `sprite2model serve` (which converts through its
// /api/convert) and published as the GitHub page (which converts with sprite2model.wasm, built there).
package web

import "embed"

// Files are the page's files.
//
//go:embed index.html app.js style.css favicon.svg examples
var Files embed.FS

package web

import "embed"

//go:embed frontend/dist
var frontendDist embed.FS

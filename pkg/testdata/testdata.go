package testdata

import (
	"embed"
)

//go:embed *.jsonnet
var TestDataFS embed.FS

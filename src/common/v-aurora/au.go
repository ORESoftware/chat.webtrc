// source file path: ./src/common/v-aurora/au.go
package au

import (
	"github.com/logrusorgru/aurora/v4"
	"os"
)

var colors = os.Getenv("vibe_with_color")
var hasColor = colors != "no"

var Col = aurora.New(aurora.WithColors(hasColor))

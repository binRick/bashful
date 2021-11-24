package utils

import (
	"embed"
	"fmt"
	"os"
)

//go:embed title.txt
var title embed.FS

func init() {
	fmt.Fprintf(os.Stderr, "Embedded Title: %s\n", title)

}

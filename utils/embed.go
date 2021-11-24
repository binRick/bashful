package utils

import (
	"embed"
)

//     xxxxxxxgo:embed title.txt
var title embed.FS

func init() {
	//	fmt.Fprintf(os.Stderr, "Embedded Title: %s\n", title)

}

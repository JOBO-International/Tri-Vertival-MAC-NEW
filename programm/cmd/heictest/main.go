package main

import (
	"fmt"
	"os"

	"github.com/jdeng/goheif"
)

func main() {
	f, _ := os.Open(os.Args[1])
	img, err := goheif.Decode(f)
	if err != nil {
		fmt.Println("Fehler:", err)
		return
	}
	f.Seek(0, 0)
	ex, err := goheif.ExtractExif(f)
	fmt.Println("HEIC ok", img.Bounds(), "Exif-Bytes:", len(ex), err)
}

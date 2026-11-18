package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"vertical3/dng"
)

func main() {
	b, _ := os.ReadFile(os.Args[1])
	f, _ := os.Create("/tmp/dng.prof")
	pprof.StartCPUProfile(f)
	t := time.Now()
	for i := 0; i < 1; i++ {
		dng.Entwickle(b)
	}
	pprof.StopCPUProfile()
	fmt.Println("je Durchlauf", time.Since(t))
}

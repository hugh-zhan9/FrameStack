package main

import (
	"flag"
	"fmt"
	"log"

	"idea/internal/devsample"
)

func main() {
	outDir := flag.String("out", "./tmp/dev-media", "output directory for generated sample media")
	flag.Parse()

	report, err := devsample.Generate(*outDir)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("sample root: %s\n", report.RootPath)
	fmt.Printf("file count: %d\n", report.FileCount)
	fmt.Printf("duplicate pair:\n")
	fmt.Printf("  - %s\n", report.DuplicatePair[0])
	fmt.Printf("  - %s\n", report.DuplicatePair[1])
}

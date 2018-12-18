# watcher

## What is it?
A Go library for watching files in a given directory.

## How does it work?
It is pretty basic at the moment. The library provides an object that scans on
an interval.

## API
The `Watcher` interface provides a means for controlling a single instance of
a file watcher. A new Watcher instance can be created using `NewWatcher()`. The
following example will create a Watcher that reports changes to files ending
in `.txt`:
```go
package main

import (
	"log"

	"github.com/stephen-fox/watcher"
)

func main() {
	watcherConfig := watcher.Config{
		ScanFunc:     watcher.ScanFilesInDirectory,
		RootDirPath:  "/tmp",
		FileSuffixes: []string{
			".txt",
		},
		Changes: make(chan watcher.Change),
	}
	
	txtWatcher, err := watcher.NewWatcher(watcherConfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	
	txtWatcher.Start()
	
	for c := range watcherConfig.Changes {
		for _, u := range c.UpdatedFilePaths() {
			log.Println("Updated '" + u + "'")
		}
		
		for _, d := range c.DeletedFilePaths() {
			log.Println("Deleted '" + d + "'")
		}
	}
}
```

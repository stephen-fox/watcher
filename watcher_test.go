package watcher

import (
	"os"
	"path"
	"testing"
	"time"
)

func TestConfig_IsValid(t *testing.T) {
	emptyErr := Config{}.IsValid()
	if emptyErr == nil {
		t.Error("Empty config did not generate an error")
	}

	noFileExtensionErr := Config{
		RootDirPath: "/bla",
		Changes:     make(chan Changes),
		ScanFunc:    ScanFilesInDirectory,
	}.IsValid()
	if noFileExtensionErr == nil {
		t.Error("Empty file extension did not generate an error")
	}

	noDirErr := Config{
		FileSuffix: ".bla",
		Changes:    make(chan Changes),
		ScanFunc:   ScanFilesInDirectory,
	}.IsValid()
	if noDirErr == nil {
		t.Error("Empty directory path did not generate an error")
	}

	noChannelErr := Config{
		RootDirPath: "fsfds",
		FileSuffix:  ".akdka",
		ScanFunc:    ScanFilesInDirectory,
	}.IsValid()
	if noChannelErr == nil {
		t.Error("Empty Changes channel did not generate an error")
	}

	noScanFuncErr := Config{
		RootDirPath: "fsfds",
		FileSuffix:  ".akdka",
		Changes:     make(chan Changes),
	}.IsValid()
	if noScanFuncErr == nil {
		t.Error("Empty scan func did not generate an error")
	}

	err := Config{
		RootDirPath: "fdf",
		FileSuffix:  ".bla",
		Changes:     make(chan Changes),
		ScanFunc:    ScanFilesInDirectory,
	}.IsValid()
	if err != nil {
		t.Error("Valid config generated an error -", err.Error())
	}
}

func TestNewWatcher(t *testing.T) {
	config := Config{}
	_, err := NewWatcher(config)
	if err == nil {
		t.Error("Empty config did not generate an error")
	}

	noFileExtensionErr := Config{
		RootDirPath: "/bla",
		Changes:     make(chan Changes),
		ScanFunc:    ScanFilesInDirectory,
	}.IsValid()
	if noFileExtensionErr == nil {
		t.Error("Empty file extension did not generate an error")
	}

	config = Config{
		FileSuffix: ".bla",
		Changes:    make(chan Changes),
		ScanFunc:   ScanFilesInDirectory,

	}
	_, err = NewWatcher(config)
	if err == nil {
		t.Error("Empty directory path did not generate an error")
	}

	config = Config{
		RootDirPath: "fsfds",
		FileSuffix:  ".akdka",
		ScanFunc:    ScanFilesInDirectory,

	}
	_, err = NewWatcher(config)
	if err == nil {
		t.Error("Empty Changes channel did not generate an error")
	}

	config = Config{
		RootDirPath: "fsfds",
		FileSuffix:  ".akdka",
	}
	_, err = NewWatcher(config)
	if err == nil {
		t.Error("Empty scan func did not generate an error")
	}

	config = Config{
		RootDirPath: "fdf",
		FileSuffix:  ".bla",
		Changes:     make(chan Changes),
		ScanFunc:    ScanFilesInDirectory,
	}
	_, err = NewWatcher(config)
	if err != nil {
		t.Error("Valid config generated an error -", err.Error())
	}
}

func TestDefaultWatcher_Start(t *testing.T) {
	current, err := os.Getwd()
	if err != nil {
		t.Error(err.Error())
	}

	config := Config{
		RefreshDelay: 1 * time.Second,
		RootDirPath:  current,
		FileSuffix:   ".go",
		Changes:      make(chan Changes),
		ScanFunc:     ScanFilesInDirectory,
	}
	w, err := NewWatcher(config)
	if err != nil {
		t.Error(err.Error())
	}
	defer w.Stop()

	w.Start()

	changes := <-config.Changes
	if changes.IsErr() {
		t.Error(changes.Err)
	}

	exp := []string{
		"doc.go",
		"watcher.go",
		"watcher_test.go",
	}

	if len(changes.UpdatedFilePaths) == 0 {
		t.Error("Updated file paths should not be empty")
	}

	OUTER:
	for _, filePath := range changes.UpdatedFilePaths {
		for _, e := range exp {
			if path.Base(filePath) == e {
				continue OUTER
			}
		}

		t.Error("Got unexpected file path -", filePath)
	}
}

func TestDefaultWatcher_StartMultipleTimes(t *testing.T) {
	current, err := os.Getwd()
	if err != nil {
		t.Error(err.Error())
	}

	config := Config{
		RefreshDelay: 1 * time.Second,
		RootDirPath:  current,
		FileSuffix:   ".go",
		Changes:      make(chan Changes),
		ScanFunc:     ScanFilesInDirectory,
	}
	w, err := NewWatcher(config)
	if err != nil {
		t.Error("Valid config generated an error -", err.Error())
	}
	defer w.Stop()

	w.Start()
	w.Start()
	w.Start()
	w.Start()
	w.Start()

	ticker := time.NewTicker(config.RefreshDelay * 2)
	defer ticker.Stop()

	var count int
	OUTER:
	for {
		select {
		case <-config.Changes:
			count++
		case <-ticker.C:
			break OUTER
		}
	}

	if count != 1 {
		t.Error("Did not receive expected changes -", count)
	}
}

func TestDefaultWatcher_Stop(t *testing.T) {
	current, err := os.Getwd()
	if err != nil {
		t.Error(err.Error())
	}

	config := Config{
		RefreshDelay: 1 * time.Second,
		RootDirPath:  current,
		FileSuffix:   ".go",
		Changes:      make(chan Changes),
		ScanFunc:     ScanFilesInDirectory,
	}
	w, err := NewWatcher(config)
	if err != nil {
		t.Error("Valid config generated an error -", err.Error())
	}

	w.Start()

	w.Stop()

	ticker := time.NewTicker(config.RefreshDelay * 2)
	defer ticker.Stop()

	var count int
	OUTER:
	for {
		select {
		case <-ticker.C:
			break OUTER
		case <-config.Changes:
			count++
		}
	}

	if count > 0 {
		t.Error("Results were produced after stopping -", count)
	}
}

func TestDefaultWatcher_StopMultipleTimes(t *testing.T) {
	current, err := os.Getwd()
	if err != nil {
		t.Error(err.Error())
	}

	config := Config{
		RefreshDelay: 1 * time.Second,
		RootDirPath:  current,
		FileSuffix:   ".go",
		Changes:      make(chan Changes),
		ScanFunc:     ScanFilesInDirectory,
	}
	w, err := NewWatcher(config)
	if err != nil {
		t.Error("Valid config generated an error -", err.Error())
	}

	w.Start()

	w.Stop()
	w.Stop()
	w.Stop()
	w.Stop()
	w.Stop()

	ticker := time.NewTicker(config.RefreshDelay * 2)
	defer ticker.Stop()

	var count int
	OUTER:
	for {
		select {
		case <-ticker.C:
			break OUTER
		case <-config.Changes:
			count++
		}
	}

	if count > 0 {
		t.Error("More than one result was produced after stopping multiple times -", count)
	}
}

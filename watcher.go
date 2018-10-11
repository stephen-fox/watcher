package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const (
	defaultRefreshDelay = 10 * time.Second
)

type Watcher interface {
	Start()
	Stop()
}

type defaultWatcher struct {
	config Config
	last   Scan
	stop   chan struct{}
}

func (o *defaultWatcher) Start() {
	if o.stop != nil {
		return
	}

	o.stop = make(chan struct{})

	go o.loop()
}

func (o *defaultWatcher) loop() {
	delay := defaultRefreshDelay
	if o.config.RefreshDelay > 0 {
		delay = o.config.RefreshDelay
	}

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var changes Changes

			current := o.config.ScanFunc(o.config)
			if current.Err != nil {
				changes.Err = current.Err
				o.config.Changes <- changes
				continue
			}

			for filePath, sha256Hash := range current.FilePathsToSha256s {
				currentSha256, ok := o.last.FilePathsToSha256s[filePath]
				if ok && sha256Hash == currentSha256 {
					continue
				}

				changes.UpdatedFilePaths = append(changes.UpdatedFilePaths, filePath)
			}

			for filePath := range o.last.FilePathsToSha256s {
				_, ok := current.FilePathsToSha256s[filePath]
				if !ok {
					changes.DeletedFilePaths = append(changes.DeletedFilePaths, filePath)
				}
			}

			o.last = current

			o.config.Changes <- changes
		case <-o.stop:
			return
		}
	}
}

func (o *defaultWatcher) Stop() {
	if o.stop != nil {
		close(o.stop)
	}

	o.stop = nil
}

type Scan struct {
	Err                error
	FilePathsToSha256s map[string]string
}

type Config struct {
	ScanFunc     func(config Config) Scan
	RefreshDelay time.Duration
	RootDirPath  string
	FileSuffix   string
	Changes      chan Changes
}

func (o Config) IsValid() error {
	if len(strings.TrimSpace(o.RootDirPath)) == 0 {
		return errors.New("The specified directory path cannot not be empty")
	}

	if len(strings.TrimSpace(o.FileSuffix)) == 0 {
		return errors.New("The specified file extension to match cannot not be empty")
	}

	if o.Changes == nil {
		return errors.New("The results channel cannot be nil")
	}

	if o.ScanFunc == nil {
		return errors.New("The scan function cannot be nil")
	}

	return nil
}

type Changes struct {
	Err              error
	UpdatedFilePaths []string
	DeletedFilePaths []string
}

func (o Changes) IsErr() bool {
	return o.Err != nil
}

// ScanFilesInDirectory scans a directory for files ending with a particular
// suffix.
//
// Consider the following file tree:
//	My Files/
//	|
//	|-- SomeFile.txt
//	|
//	|-- Awesome.cfg
//	|
//	|-- gorbage/
//	   |
//	   |-- CoolStoryBro.txt
//
// If you specify the root directory to scan as 'My Files', and the file suffix
// as '.cfg', the function will return a map of file paths to hashes containing
// 'path/to/My Files/Awesome.cfg'.
func ScanFilesInDirectory(config Config) Scan {
	subInfos, err := ioutil.ReadDir(config.RootDirPath)
	if err != nil {
		return Scan{
			Err: err,
		}
	}

	result := Scan{
		FilePathsToSha256s: make(map[string]string),
	}

	for _, sub := range subInfos {
		if sub.IsDir() || !strings.HasSuffix(sub.Name(), config.FileSuffix) {
			continue
		}

		filePath := path.Join(config.RootDirPath, sub.Name())

		sha256Hash, err := getFileSha256(filePath)
		if err != nil {
			// TODO: Do something about the error.
			continue
		}

		result.FilePathsToSha256s[filePath] = sha256Hash
	}

	return result
}

// ScanFilesInSubdirectories scans a directory's subdirectories for files
// with a particular suffix.
//
// Consider the following file tree:
//	My Files/
//	|
//	|-- text-files/
//	|  |
//	|  |-- SomeFile.txt
//	|
//	|-- stuff/
//	|  |
//	|  |-- Awesome.cfg
//	|
//	|-- gorbage/
//	   |
//	   |-- CoolStoryBro.txt
//
// If you specify the root directory to scan as 'My Files', and the file suffix
// as '.cfg', the function will return a map of file paths to hashes containing
// 'path/to/My Files/stuff/Awesome.cfg'.
func ScanFilesInSubdirectories(config Config) Scan {
	subInfos, err := ioutil.ReadDir(config.RootDirPath)
	if err != nil {
		return Scan{
			Err: err,
		}
	}

	result := Scan{
		FilePathsToSha256s: make(map[string]string),
	}

	for _, sub := range subInfos {
		if !sub.IsDir() {
			continue
		}

		subDirPath := path.Join(config.RootDirPath, sub.Name())

		children, childErr := ioutil.ReadDir(subDirPath)
		if childErr != nil {
			continue
		}

		for _, c := range children {
			if c.IsDir() || !strings.HasSuffix(c.Name(), config.FileSuffix) {
				continue
			}

			cPath := path.Join(subDirPath, c.Name())

			h, err := getFileSha256(cPath)
			if err != nil {
				// TODO: Do something about the error.
				continue
			}

			result.FilePathsToSha256s[cPath] = h
		}
	}

	return result
}

func getFileSha256(filePath string) (string, error) {
	return getFileHash(filePath, sha256.New())
}

func getFileHash(filePath string, hash hash.Hash) (string, error) {
	target, err := os.OpenFile(filePath, os.O_RDONLY, os.ModeAppend)
	if err != nil {
		return "", err
	}
	defer target.Close()

	_, err = io.Copy(hash, target)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func NewWatcher(config Config) (Watcher, error) {
	err := config.IsValid()
	if err != nil {
		return &defaultWatcher{}, err
	}

	return &defaultWatcher{
		config: config,
	}, nil
}

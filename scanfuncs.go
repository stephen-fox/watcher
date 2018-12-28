package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

// ScanResult provides information about the result of a scan for
// modified files.
type ScanResult struct {
	FilePathsToInfo map[string]MatchInfo
}

// MatchInfo provides information about a single modified file that met the
// match criteria.
type MatchInfo struct {
	Path      string
	ModTime   time.Time
	MatchedOn string
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
// as '.cfg', the function will return a ScanResult containing
// 'path/to/My Files/Awesome.cfg'.
func ScanFilesInDirectory(config Config) (ScanResult, error) {
	subInfos, err := ioutil.ReadDir(config.RootDirPath)
	if err != nil {
		return ScanResult{}, &ScanError{
			reason:         err.Error(),
			rootReadFailed: true,
		}
	}

	result := ScanResult{
		FilePathsToInfo: make(map[string]MatchInfo),
	}

	for _, sub := range subInfos {
		if sub.IsDir() {
			continue
		}

		suffix, matches := matchesSuffixes(sub.Name(), config.FileSuffixes)
		if !matches {
			continue
		}

		filePath := path.Join(config.RootDirPath, sub.Name())

		result.FilePathsToInfo[filePath] = MatchInfo{
			Path:      filePath,
			MatchedOn: suffix,
			ModTime:   sub.ModTime(),
		}
	}

	return result, nil
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
// as '.cfg', the function will return a ScanResult containing
// 'path/to/My Files/stuff/Awesome.cfg'.
func ScanFilesInSubdirectories(config Config) (ScanResult, error) {
	subInfos, err := ioutil.ReadDir(config.RootDirPath)
	if err != nil {
		return ScanResult{}, &ScanError{
			reason:         err.Error(),
			rootReadFailed: true,
		}
	}

	result := ScanResult{
		FilePathsToInfo: make(map[string]MatchInfo),
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
			if c.IsDir() {
				continue
			}

			suffix, matches := matchesSuffixes(c.Name(), config.FileSuffixes)
			if !matches {
				continue
			}

			cPath := path.Join(subDirPath, c.Name())

			result.FilePathsToInfo[cPath] = MatchInfo{
				Path:      cPath,
				MatchedOn: suffix,
				ModTime:   c.ModTime(),
			}
		}
	}

	return result, nil
}

func matchesSuffixes(s string, suffixes []string) (string, bool) {
	for i := range suffixes {
		if strings.HasSuffix(s, suffixes[i]) {
			return suffixes[i], true
		}
	}

	return "", false
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

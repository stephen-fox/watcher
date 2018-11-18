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
)

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

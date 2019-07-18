package watcher

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	defaultRefreshDelay = 10 * time.Second
)

const (
	updated changeState = "updated"
	deleted changeState = "deleted"
)

type changeState string

// Watcher provides an interface for controlling a file watcher.
type Watcher interface {
	// Start starts the Watcher.
	Start()

	// Stop stops the Watcher.
	Stop()

	// Destroy stops the Watcher and closes the Config.Changes channel.
	// This should only be called if you do not intend to use the Watcher.
	Destroy()

	// Config returns the Watcher's Config.
	Config() *Config
}

type defaultWatcher struct {
	mutex  *sync.Mutex
	config Config
	last   ScanResult
	stop   chan struct{}
	kill   chan struct{}
}

func (o *defaultWatcher) Start() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	select {
	case _, open := <-o.kill:
		if !open {
			return
		}
	case _, open := <-o.stop:
		if !open {
			o.stop = make(chan struct{})
		}
	default:
		return
	}

	go o.loop(o.config)
}

func (o *defaultWatcher) loop(config Config) {
	delay := defaultRefreshDelay
	if config.RefreshDelay > 0 {
		delay = config.RefreshDelay
	}

	for {
		time.Sleep(delay)

		current, err := config.ScanFunc(config)
		change := &defaultChange{
			scanResult:  current,
			stateToInfo: make(map[changeState][]MatchInfo),
		}
		if err != nil {
			change.err = err
			config.Changes <- change
			continue
		}

		for currentFilePath, current := range current.FilePathsToInfo {
			last, exists := o.last.FilePathsToInfo[currentFilePath]
			if exists && current.ModTime == last.ModTime {
				continue
			}

			change.stateToInfo[updated] = append(change.stateToInfo[updated], current)
		}

		for lastFilePath, info := range o.last.FilePathsToInfo {
			_, ok := current.FilePathsToInfo[lastFilePath]
			if !ok {
				change.stateToInfo[deleted] = append(change.stateToInfo[deleted], info)
			}
		}

		o.last = current

		select {
		case <-o.kill:
			close(config.Changes)
			return
		case <-o.stop:
			return
		default:
			if len(change.stateToInfo) > 0 {
				config.Changes <- change
			}
		}
	}
}

func (o *defaultWatcher) Destroy() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	select {
	case _, open := <-o.kill:
		if !open {
			return
		}
	default:
	}

	close(o.kill)
}

func (o *defaultWatcher) Stop() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	select {
	case _, open := <-o.stop:
		if !open {
			return
		}
	default:
	}

	close(o.stop)
}

func (o *defaultWatcher) Config() *Config {
	return &o.config
}

// Config configures a Watcher.
type Config struct {
	// ScanFunc is the function to execute when it is time to
	// scan for a change.
	ScanFunc func(config Config) (ScanResult, error)

	// RefreshDelay is the time to wait between scans.
	RefreshDelay time.Duration

	// RootDirPath is the root directory to scan.
	RootDirPath string

	// ScanCriteria is a slice of strings that ScanFunc uses
	// to match files.
	ScanCriteria []string

	// Changes is the channel to receive a Change when a change occurs.
	Changes chan Change
}

func (o Config) IsValid() error {
	if len(strings.TrimSpace(o.RootDirPath)) == 0 {
		return errors.New("the directory path to watch cannot not be empty")
	}

	if len(o.ScanCriteria) == 0 {
		return errors.New("the file suffixes to match cannot not be empty")
	}

	if o.Changes == nil {
		return errors.New("the changes channel cannot be nil")
	}

	if o.ScanFunc == nil {
		return errors.New("the scan function cannot be nil")
	}

	return nil
}

// Change provides an interface for retrieving information about
// changes that occurred.
type Change interface {
	IsErr() bool
	RootReadErr() bool
	ErrDetails() string
	UpdatedFilePaths() []string
	DeletedFilePaths() []string
	UpdatedFilePathsWithSuffixes(suffixes []string) []string
	DeletedFilePathsWithSuffixes(suffixes []string) []string
	UpdatedFilePathsWithoutSuffixes(suffixes []string) []string
	DeletedFilePathsWithoutSuffixes(suffixes []string) []string
}

type defaultChange struct {
	err         error
	scanResult  ScanResult
	stateToInfo map[changeState][]MatchInfo
}

func (o *defaultChange) IsErr() bool {
	return o.err != nil
}

func (o *defaultChange) RootReadErr() bool {
	if o.err != nil {
		sErr, is := o.err.(ScanError)
		if is {
			if sErr.RootDirectoryReadFailed() {
				return true
			}
		}
	}

	return false
}

func (o *defaultChange) ErrDetails() string {
	if o.err != nil {
		return o.err.Error()
	}

	return ""
}

func (o *defaultChange) UpdatedFilePaths() []string {
	var r []string

	for _, c := range o.stateToInfo[updated] {
		r = append(r, c.Path)
	}

	return r
}

func (o *defaultChange) DeletedFilePaths() []string {
	var r []string

	for _, c := range o.stateToInfo[deleted] {
		r = append(r, c.Path)
	}

	return r
}

func (o *defaultChange) UpdatedFilePathsWithSuffixes(suffixes []string) []string {
	var r []string

	for _, c := range o.stateToInfo[updated] {
		for i := range suffixes {
			if c.MatchedOn == suffixes[i] {
				r = append(r, c.Path)
				break
			}
		}
	}

	return r
}

func (o *defaultChange) DeletedFilePathsWithSuffixes(suffixes []string) []string {
	var r []string

	for _, c := range o.stateToInfo[deleted] {
		for i := range suffixes {
			if c.MatchedOn == suffixes[i] {
				r = append(r, c.Path)
				break
			}
		}
	}

	return r
}

func (o *defaultChange) UpdatedFilePathsWithoutSuffixes(suffixes []string) []string {
	var r []string

OUTER:
	for _, c := range o.stateToInfo[updated] {
		for i := range suffixes {
			if c.MatchedOn == suffixes[i] {
				continue OUTER
			}
		}
		r = append(r, c.Path)
	}

	return r
}

func (o *defaultChange) DeletedFilePathsWithoutSuffixes(suffixes []string) []string {
	var r []string

OUTER:
	for _, c := range o.stateToInfo[deleted] {
		for i := range suffixes {
			if c.MatchedOn == suffixes[i] {
				continue OUTER
			}
		}
		r = append(r, c.Path)
	}

	return r
}

// NewWatcher creates a new Watcher for the provided Config.
func NewWatcher(config Config) (Watcher, error) {
	err := config.IsValid()
	if err != nil {
		return &defaultWatcher{}, err
	}

	w := &defaultWatcher{
		mutex:  &sync.Mutex{},
		config: config,
		kill:   make(chan struct{}),
		stop:   make(chan struct{}),
	}

	close(w.stop)

	return w, nil
}

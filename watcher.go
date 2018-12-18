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

type Watcher interface {
	Start()
	Stop()
	Destroy()
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

		current := config.ScanFunc(config)
		change := &defaultChange{
			scanResult:  current,
			stateToInfo: make(map[changeState][]MatchInfo),
		}
		if current.Err != nil {
			config.Changes <- change
			continue
		}

		for currentFilePath, current := range current.FilePathsToInfo {
			last, exists := o.last.FilePathsToInfo[currentFilePath]
			if exists && current.Hash == last.Hash {
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

type Config struct {
	ScanFunc     func(config Config) ScanResult
	RefreshDelay time.Duration
	RootDirPath  string
	FileSuffixes []string
	Changes      chan Change
}

func (o Config) IsValid() error {
	if len(strings.TrimSpace(o.RootDirPath)) == 0 {
		return errors.New("the directory path to watch cannot not be empty")
	}

	if len(o.FileSuffixes) == 0 {
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
	scanResult  ScanResult
	stateToInfo map[changeState][]MatchInfo
}

func (o *defaultChange) IsErr() bool {
	return o.scanResult.Err != nil
}

func (o *defaultChange) RootReadErr() bool {
	return o.scanResult.RootReadFailed
}

func (o *defaultChange) ErrDetails() string {
	if o.scanResult.Err != nil {
		return o.scanResult.Err.Error()
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

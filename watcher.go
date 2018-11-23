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
		changes := &defaultChanges{
			scanResult: current,
		}
		if current.Err != nil {
			config.Changes <- changes
			continue
		}

		for filePath, currentSha256 := range current.FilePathsToSha256s {
			lastSha256, exists := o.last.FilePathsToSha256s[filePath]
			if exists && currentSha256 == lastSha256 {
				continue
			}

			changes.updatedFilePaths = append(changes.updatedFilePaths, filePath)
		}

		for filePath := range o.last.FilePathsToSha256s {
			_, ok := current.FilePathsToSha256s[filePath]
			if !ok {
				changes.deletedFilePaths = append(changes.deletedFilePaths, filePath)
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
			config.Changes <- changes
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

type ScanResult struct {
	Err                error
	RootReadFailed     bool
	FilePathsToSha256s map[string]string
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
		return errors.New("The directory path to watch cannot not be empty")
	}

	if len(o.FileSuffixes) == 0 {
		return errors.New("The file suffixes to match cannot not be empty")
	}

	if o.Changes == nil {
		return errors.New("The changes channel cannot be nil")
	}

	if o.ScanFunc == nil {
		return errors.New("The scan function cannot be nil")
	}

	return nil
}

type Change interface {
	IsErr() bool
	RootReadErr() bool
	ErrDetails() string
	UpdatedFilePaths() []string
	DeletedFilePaths() []string
}

type defaultChanges struct {
	scanResult       ScanResult
	updatedFilePaths []string
	deletedFilePaths []string
}

func (o *defaultChanges) IsErr() bool {
	return o.scanResult.Err != nil
}

func (o *defaultChanges) RootReadErr() bool {
	return o.scanResult.RootReadFailed
}

func (o *defaultChanges) ErrDetails() string {
	if o.scanResult.Err != nil {
		return o.scanResult.Err.Error()
	}

	return ""
}

func (o *defaultChanges) UpdatedFilePaths() []string {
	return o.updatedFilePaths
}

func (o *defaultChanges) DeletedFilePaths() []string {
	return o.deletedFilePaths
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

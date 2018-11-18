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
	last   Scan
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

		var changes Changes

		current := config.ScanFunc(config)
		if current.Err != nil {
			changes.Err = current.Err
			config.Changes <- changes
			continue
		}

		for filePath, currentSha256 := range current.FilePathsToSha256s {
			lastSha256, exists := o.last.FilePathsToSha256s[filePath]
			if exists && currentSha256 == lastSha256 {
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
		return errors.New("The directory path to watch cannot not be empty")
	}

	if len(strings.TrimSpace(o.FileSuffix)) == 0 {
		return errors.New("The file suffix to match cannot not be empty")
	}

	if o.Changes == nil {
		return errors.New("The changes channel cannot be nil")
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

package watcher

type ScanError struct {
	reason         string
	rootReadFailed bool
}

func (o ScanError) Error() string {
	return o.reason
}

func (o ScanError) RootDirectoryReadFailed() bool {
	return o.rootReadFailed
}

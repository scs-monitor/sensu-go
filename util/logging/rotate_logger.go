package logging

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type renameFunc func(string, string) error

var rename renameFunc = os.Rename

type RotateFileWriterConfig struct {
	Path              string
	MaxSizeBytes      int64
	RetentionDuration time.Duration
	RetentionFiles    int64

	sync bool // for testing only
}

func (f *fileReplacer) archive(archiveName string) error {
	reader, err := os.Open(archiveName)
	if err != nil {
		return err
	}
	zipFile, err := os.Create(archiveName + ".zip")
	if err != nil {
		return err
	}
	defer zipFile.Close()
	zipper := zip.NewWriter(zipFile)
	zipWriter, err := zipper.Create(archiveName)
	if err != nil {
		return err
	}
	if _, err := io.Copy(zipWriter, reader); err != nil {
		return err
	}
	if err := zipper.Flush(); err != nil {
		return err
	}
	if err := os.Remove(archiveName); err != nil {
		return err
	}
	return nil
}

type fileReplacer struct {
	count     int64
	max       int64
	once      sync.Once
	container *atomic.Value
	file      *os.File
	sync      bool // only for testing purposes
}

func (f *fileReplacer) Rotate() (*fileReplacer, error) {
	now := time.Now().UnixNano()
	currentName := f.file.Name()
	archiveName := fmt.Sprintf("%s.%d", currentName, now)

	if err := f.file.Close(); err != nil {
		return nil, err
	}

	if err := rename(currentName, archiveName); err != nil {
		return f, err
	}
	replacement := &fileReplacer{
		max:       f.max,
		container: f.container,
		sync:      f.sync,
	}
	var err error
	replacement.file, err = os.OpenFile(currentName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return f, err
	}

	if f.sync {
		fmt.Println("!")
		if err := f.archive(archiveName); err != nil {
			return nil, err
		}
	} else {
		fmt.Println("?")
		// archiver errors are silently ignored in production,
		// as there is nothing that can be done about them.
		go f.archive(archiveName)
	}

	return replacement, nil
}

func (f *fileReplacer) Write(p []byte) (int, error) {
	projected := atomic.AddInt64(&f.count, int64(len(p)))
	if projected <= f.max {
		return f.file.Write(p)
	}
	var err error
	f.once.Do(func() {
		var fr *fileReplacer
		fr, err = f.Rotate()
		if err == nil {
			f.container.Store(fr)
		}
	})
	if err != nil {
		return 0, fmt.Errorf("error rotating log: %s", err)
	}
	replacement := f.container.Load().(*fileReplacer)
	if replacement == f {
		return 0, errors.New("error rotating log")
	}
	return replacement.Write(p)
}

func (f *fileReplacer) Close() error {
	return f.file.Close()
}

type RotateFileWriter struct {
	retentionFiles    int64
	closed            int64
	retentionDuration time.Duration
	container         *atomic.Value
	path              string
}

func NewRotateFileWriter(cfg RotateFileWriterConfig) (*RotateFileWriter, error) {
	if cfg.Path == "" {
		cfg.Path = fmt.Sprintf("%s.log", os.Args[0])
	}
	if cfg.MaxSizeBytes == 0 {
		// 128 MB
		cfg.MaxSizeBytes = 1 << 27
	}
	w := &RotateFileWriter{
		path:              cfg.Path,
		retentionDuration: cfg.RetentionDuration,
		retentionFiles:    cfg.RetentionFiles,
		container:         new(atomic.Value),
	}
	var count int64
	fi, err := os.Stat(cfg.Path)
	if err == nil {
		count = fi.Size()
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	// Open the log file for writing in append mode whether or not it exists
	f, ferr := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if ferr != nil {
		err = ferr
	}
	fr := &fileReplacer{
		file:      f,
		max:       cfg.MaxSizeBytes,
		count:     count,
		container: w.container,
		sync:      cfg.sync,
	}
	w.container.Store(fr)
	return w, nil
}

func (r *RotateFileWriter) StartReaper(ctx context.Context, interval time.Duration) <-chan error {
	errors := make(chan error, 1)
	go r.reapLoop(ctx, errors, interval)
	return errors
}

func (r *RotateFileWriter) reapLoop(ctx context.Context, errors chan error, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(errors)
			return
		case <-ticker.C:
			errors <- r.reap()
		}
	}
}

func (r *RotateFileWriter) reap() error {
	base := filepath.Dir(r.path)
	f, err := os.Open(base)
	if err != nil {
		return err
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		return err
	}
	filesToReap := make([]string, 0, len(files))
	reapRegexp := regexp.MustCompile(fmt.Sprintf(`^%s\.(\d+)\.zip$`, regexp.QuoteMeta(r.path)))
	for _, file := range files {
		if reapRegexp.MatchString(file) {
			filesToReap = append(filesToReap, file)
		}
	}
	tooOld := make(map[string]bool, len(filesToReap))
	if r.retentionDuration > 0 {
		for _, file := range filesToReap {
			matches := reapRegexp.FindStringSubmatch(file)
			if len(matches) < 2 {
				continue
			}
			var timestamp int64
			if _, err := fmt.Sscanf(matches[1], "%d", timestamp); err != nil {
				continue
			}
			archiveTime := time.Unix(timestamp, 0)
			if archiveTime.Add(r.retentionDuration).Before(time.Now()) {
				tooOld[file] = true
				if err := os.Remove(file); err != nil {
					return err
				}
			}
		}
	}
	notTooOld := make([]string, 0, len(filesToReap))
	for _, file := range filesToReap {
		if !tooOld[file] {
			notTooOld = append(notTooOld, file)
		}
	}
	if r.retentionFiles > 0 && int64(len(notTooOld)) > r.retentionFiles {
		sort.Strings(notTooOld)
		toRemove := notTooOld[r.retentionFiles:]
		for _, file := range toRemove {
			if err := os.Remove(file); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RotateFileWriter) Write(p []byte) (int, error) {
	writer := r.container.Load().(*fileReplacer)
	return writer.Write(p)
}

func (r *RotateFileWriter) Close() error {
	atomic.StoreInt64(&r.closed, 1)
	return r.container.Load().(*fileReplacer).Close()
}

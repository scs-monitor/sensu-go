package logging

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRotateFileWriter(t *testing.T) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(td)
	}()
	logPath := filepath.Join(td, "sensu.log")

	var wg, writeWg sync.WaitGroup
	writeWg.Add(100)
	wg.Add(99)

	config := RotateFileWriterConfig{
		Path:           logPath,
		MaxSizeBytes:   1024,
		RetentionFiles: 10,
		// sync is to cause the writer to block on zipping the file, which is
		// useful for testing, but not desirable for production use.
		sync: true,
	}

	writer, err := NewRotateFileWriter(config)
	if err != nil {
		t.Fatal(err)
	}

	msg := make([]byte, 512)
	for i := range msg {
		msg[i] = '!'
	}

	for i := 0; i < 100; i++ {
		go func(i int) {
			defer writeWg.Done()
			writer.Write(msg)
		}(i)
	}

	writeWg.Wait()

	dir, err := os.Open(td)
	if err != nil {
		t.Fatal(err)
	}
	names, err := dir.Readdirnames(0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(names), 50; got != want {
		t.Errorf("wrong number of log files: got %d, want %d", got, want)
	}
	// ctx, cancel = context.WithCancel(context.Background())
	// ch := writer.StartReaper(ctx, time.Millisecond*100)

}

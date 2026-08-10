package recording

import (
	"encoding/binary"
	"math"
	"os"
	"sync"
	"time"
)

const (
	sampleRate    = 16000
	channels      = 1
	bitsPerSample = 16
)

type Recorder struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	started time.Time
	samples int64
	closed  bool
}

func NewRecorder(path string) (*Recorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := writeWavHeader(f, 0); err != nil {
		f.Close()
		return nil, err
	}
	return &Recorder{
		file:    f,
		path:    path,
		started: time.Now(),
	}, nil
}

func (r *Recorder) WritePCM(pcm []float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.file == nil {
		return
	}
	buf := make([]byte, len(pcm)*2)
	for i, s := range pcm {
		v := floatToInt16(s)
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	n, _ := r.file.Write(buf)
	r.samples += int64(n / 2)
}

func (r *Recorder) Stop() (path string, duration time.Duration, fileSize int64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.path, 0, 0, nil
	}
	r.closed = true
	duration = time.Since(r.started)
	if r.file != nil {
		_, _ = r.file.Seek(0, 0)
		_ = writeWavHeader(r.file, r.samples)
		fi, _ := r.file.Stat()
		if fi != nil {
			fileSize = fi.Size()
		}
		_ = r.file.Close()
		r.file = nil
	}
	return r.path, duration, fileSize, nil
}

func (r *Recorder) Duration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0
	}
	return time.Since(r.started)
}

func writeWavHeader(f *os.File, totalSamples int64) error {
	dataSize := totalSamples * (bitsPerSample / 8)
	fileSize := 36 + dataSize
	buf := make([]byte, 44)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], channels)
	binary.LittleEndian.PutUint32(buf[24:28], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*channels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(channels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	_, err := f.Write(buf)
	return err
}

func floatToInt16(s float32) int16 {
	switch {
	case math.IsNaN(float64(s)):
		return 0
	case s >= 1:
		return math.MaxInt16
	case s <= -1:
		return math.MinInt16
	}
	return int16(s * 32767)
}

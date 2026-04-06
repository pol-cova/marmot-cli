package backup

import (
	"compress/gzip"
	"fmt"
	"io"
)

// Compressor handles gzip compression
type Compressor struct {
	level int
}

// NewCompressor creates a new compressor with compression level 9
func NewCompressor() *Compressor {
	return &Compressor{level: gzip.BestCompression}
}

// Compress compresses data from reader and writes to writer
func (c *Compressor) Compress(r io.Reader, w io.Writer) error {
	gzWriter, err := gzip.NewWriterLevel(w, c.level)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzWriter.Close()
	
	if _, err := io.Copy(gzWriter, r); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	
	return nil
}

// Decompress decompresses data from reader and writes to writer
func (c *Compressor) Decompress(r io.Reader, w io.Writer) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()
	
	if _, err := io.Copy(w, gzReader); err != nil {
		return fmt.Errorf("failed to decompress data: %w", err)
	}
	
	return nil
}


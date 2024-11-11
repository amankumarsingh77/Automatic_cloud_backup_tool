package filesystem

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func CompressFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	compressedFile, err := os.Create(filepath.Base(filePath) + ".gzip")
	if err != nil {
		return "", err
	}
	defer compressedFile.Close()
	gzipWriter := gzip.NewWriter(compressedFile)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, compressedFile)
	if err != nil {
		return "", err
	}
	gzipWriter.Flush()
	return filepath.Base(filePath) + ".gz", nil
}

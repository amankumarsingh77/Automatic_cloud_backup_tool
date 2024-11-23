package filesystem

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func CompressFile(folderPath string) (string, error) {
	compressedFile, err := os.Create(filepath.Base(folderPath) + ".gz")
	if err != nil {
		return "", fmt.Errorf("could not create a compressed file : %v", err.Error())
	}
	defer compressedFile.Close()

	gzipWriter := gzip.NewWriter(compressedFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == folderPath {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		relpath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}

		header := &tar.Header{
			Name: relpath,
			Size: info.Size(),
			Mode: int64(info.Mode()),
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		_, err = io.Copy(tarWriter, file)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("could not compress file : %v", err.Error())
	}
	return filepath.Base(folderPath) + ".gz", nil
}

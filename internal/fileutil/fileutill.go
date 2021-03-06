// Package fileutil provide functionality for working with files and directories.
package fileutil

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// WriteFile creates file with passed name at passed dir and writes data.
func WriteFile(fname, dir string, data string) error {
	if ext := filepath.Ext(filepath.Base(fname)); ext != "" {
		fname = strings.TrimSuffix(fname, ext)
	}

	outName := filepath.Join(dir, fmt.Sprintf("%s.json", fname))

	if err := createDirIfNotExist(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	resFile, err := os.Create(outName)
	if err != nil {
		return fmt.Errorf("failed to create result file [%s]: %w", outName, err)
	}

	if _, err = resFile.WriteString(data); err != nil {
		return fmt.Errorf("failed to write result to file: %w", err)
	}

	if err = resFile.Close(); err != nil {
		return fmt.Errorf("failed to close result file: %w", err)
	}

	return nil
}

// MoveFile moves file from base dir to target.
func MoveFile(name string, sourceDir, destDir string) error {
	sourcePath := filepath.Join(sourceDir, name)

	inputFile, err := os.Open(filepath.Clean(sourcePath))
	if err != nil {
		return fmt.Errorf("couldn't open source file: %w", err)
	}

	defer func() {
		if err = inputFile.Close(); err != nil {
			log.Errorf("failed to close inputFile: %v", err)
		}
	}()

	if err = createDirIfNotExist(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destPath: %w", err)
	}

	destPath := filepath.Join(destDir, name)

	outputFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("couldn't open dest file: %w", err)
	}

	defer func() {
		if err = outputFile.Close(); err != nil {
			log.Errorf("failed to close outputFile: %v", err)
		}
	}()

	if _, err = io.Copy(outputFile, inputFile); err != nil {
		return fmt.Errorf("writing to output file failed: %w", err)
	}

	// The copy was successful, so now delete the original file
	if err = os.Remove(sourcePath); err != nil {
		return fmt.Errorf("failed removing original file: %w", err)
	}

	return nil
}

func createDirIfNotExist(dir string, mode os.FileMode) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, mode); err != nil {
			return fmt.Errorf("failed to create directory for [%s]: %w", dir, err)
		}
	}

	return nil
}

// PollDirectory will pol for files that needs to be processed.
func PollDirectory(ctx context.Context, dir string, availableExtensions map[string]bool, fileChan chan string) {
	const pollInterval int = 1

	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)

	defer func() {
		close(fileChan)
		ticker.Stop()
	}()

	log.Printf("started to watch directory <%s> for data files", dir)

	var lastFile string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			files, err := ioutil.ReadDir(dir)
			if err != nil {
				log.Error(err)

				return
			}

			for _, file := range files {
				if file.IsDir() {
					continue
				}

				ext := filepath.Ext(file.Name())
				if ext != "" {
					ext = strings.TrimPrefix(ext, ".")
				}

				if file.Name() == lastFile {
					log.Warnf("Cannot process the last known file: %s", lastFile)
				}

				_, ok := availableExtensions[ext]
				if ok && file.Name() != lastFile {
					fileChan <- file.Name()

					lastFile = file.Name()

					break
				}
			}
		}
	}
}

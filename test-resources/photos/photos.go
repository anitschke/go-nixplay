package photos

import (
	"errors"
	"io"
	"os"
	"path"
	"runtime"
)

const expPhotoCount = 9

type TestPhoto struct {
	Name     string
	FullPath string
}

func (p TestPhoto) Open() (io.ReadCloser, error) {
	return os.Open(p.FullPath)
}

func AllPhotos() ([]TestPhoto, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("failed to identify photo location")
	}
	thisFileName := path.Base(thisFile)
	thisFolder := path.Dir(thisFile)

	entries, err := os.ReadDir(thisFolder)
	if err != nil {
		return nil, err
	}

	photos := make([]TestPhoto, 0, expPhotoCount)
	for _, e := range entries {
		if e.Name() == thisFileName {
			continue
		}
		fullPath := path.Join(thisFolder, e.Name())
		p := TestPhoto{
			Name:     e.Name(),
			FullPath: fullPath,
		}
		photos = append(photos, p)
	}

	// Protect against no photos being returned at all causing potential
	// issues with our tests that depend on some photos being returned
	if len(photos) != expPhotoCount {
		return nil, errors.New("unexpected number of test photos")
	}

	return photos, nil
}

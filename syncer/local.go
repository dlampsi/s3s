package syncer

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Initializes local files md5 hash cache.
func (s *SyncerService) InitHashCache() {
	var count int

	err := filepath.Walk(s.cfg.LocalDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			flog := log.With("file", path)

			f, err := os.Open(path)
			if err != nil {
				flog.Errorw("Can't open local file for checksum calc", "err", err.Error())
				return nil
			}

			relPath, err := filepath.Rel(s.cfg.LocalDir, path)
			if err != nil {
				flog.Errorw("Can't get file relative path", "err", err.Error())
				return nil
			}

			hash, err := s.getHash(f)
			if err != nil {
				flog.Errorw("Can't calculate file checksum", "err", err.Error())
				return nil
			}

			s.cacheHashLock.Lock()
			s.cacheHash[relPath] = fmt.Sprintf("\"%s\"", hash)
			s.cacheHashLock.Unlock()

			count++

			return nil
		})
	if err != nil {
		log.Error(err)
	}

	log.Infof("Hash cache initialized for %d files", count)
}

// Returns map of all files in direcotry
func (s *SyncerService) listDir(dirPath string) (map[string]bool, error) {
	result := map[string]bool{}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var skip bool

		if _, err := filepath.Rel(dirPath, path); err != nil {
			log.Errorw("Can't get relative path", "path", path, "err", err.Error())
			skip = true
		}

		if info.IsDir() {
			if skip {
				return filepath.SkipDir
			}
		} else {
			if skip {
				return nil
			}
			result[path] = true
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *SyncerService) removeEmptyDirs(dirPath string) error {
	toDel := map[string]bool{}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var skip bool

		if _, err := filepath.Rel(dirPath, path); err != nil {
			log.Errorw("Can't get relative path", "path", path, "err", err.Error())
			skip = true
		}

		if info.IsDir() {
			if skip {
				return filepath.SkipDir
			}
			// All except root dir
			if path != dirPath {
				toDel[path] = true
			}
		} else {
			if skip {
				return nil
			}
			// Do not delete dir if files exists in it
			parent := filepath.Dir(path)
			if _, ok := toDel[parent]; ok {
				delete(toDel, parent)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	for d := range toDel {
		if err := os.Remove(d); err != nil {
			log.Errorw("can't remove dir", "err", err.Error())
		}
	}

	return nil
}

func (s *SyncerService) createTempDir() error {
	if _, err := os.Stat(s.cfg.TempDir); os.IsNotExist(err) {
		err = os.MkdirAll(s.cfg.TempDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("can't create temp dir: %v", err.Error())
		}
	}
	return nil
}

func (s *SyncerService) deleteTempDir() {
	if err := os.RemoveAll(s.cfg.TempDir); err != nil {
		log.Errorw("Can't delete temp dir", "err", err.Error())
	}
}

// Remove files that doesn't exists on remote and cleanup empty dirs.
func (s *SyncerService) housekeeper() error {
	var (
		todellist   []string
		delFilesCnt int
	)

	for k := range s.cacheDel {
		todellist = append(todellist, k)
	}

	log.Debugf("Found %d files to delete", len(todellist))

	for _, f := range todellist {
		if err := os.Remove(f); err != nil {
			log.Errorw("Can't remove file", "path", f, "err", err.Error())
		} else {
			delFilesCnt++
		}
	}
	if delFilesCnt > 0 {
		log.Infof("Deleted %d files", delFilesCnt)
	}

	// Cleanup empty local dirs
	if err := s.removeEmptyDirs(s.cfg.LocalDir); err != nil {
		return err
	}

	return nil
}

func (s *SyncerService) getHash(f *os.File) (string, error) {
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("can't calculate file checksum: %v", err.Error())
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

package syncer

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type s3Path struct {
	bucket string
	path   string
}

// Parse S3 uri to bucket and path struct.
func parseS3Path(path string) (*s3Path, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "s3" {
		return nil, errors.New("path is not valid s3 url")
	}
	if u.Host == "" {
		return nil, errors.New("empty bucket in s3 url")
	}
	result := &s3Path{
		bucket: u.Host,
		path:   u.Path,
	}

	if strings.HasPrefix(result.path, "/") {
		result.path = trimFirstRune(result.path)
	}
	return result, nil
}

func trimFirstRune(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

// Populate s3 session configuration.
func (s *SyncerService) getS3cfg() *aws.Config {
	cfg := &aws.Config{
		Region: aws.String("us-west-1"),
	}
	if s.cfg.DisableSSL {
		cfg.DisableSSL = aws.Bool(true)
	}
	if s.cfg.S3Endpoint != "" {
		cfg.Endpoint = aws.String(s.cfg.S3Endpoint)
		cfg.S3ForcePathStyle = aws.Bool(true)
	}
	return cfg
}

func (s *SyncerService) procS3(page *s3.ListObjectsV2Output, lastPage bool) bool {
	log.Debugf("Found %d objects on S3", len(page.Contents))

	for _, obj := range page.Contents {
		if obj.Key == nil {
			continue
		}
		objKey := *(obj.Key)

		if strings.HasSuffix(objKey, "/") {
			log.Debugw("Skipping directory", "key", objKey)
			continue
		}

		hash := *(obj.ETag)
		flog := log.With("hash", hash)

		uri := fmt.Sprintf("s3://%s/%s", s.cfg.s3Bucket, objKey)

		flog = flog.With("uri", uri)

		relPath, err := filepath.Rel(s.cfg.s3Prefix, objKey)
		if err != nil {
			flog.Debugf("Skip object %s is not the parent of %s\n", s.cfg.RemoteURI, objKey)
			continue
		}
		// skip parent dir
		if relPath == "" || relPath == "/" || relPath == "." {
			continue
		}

		localPath := filepath.Join(s.cfg.LocalDir, relPath)
		flog = flog.With("local_path", localPath)

		// Remove file from delete list.
		s.cacheDelLock.Lock()
		delete(s.cacheDel, localPath)
		s.cacheDelLock.Unlock()

		s.listedCount++

		s.cacheHashLock.Lock()
		oldHash, ok := s.cacheHash[relPath]
		s.cacheHashLock.Unlock()

		if ok && oldHash == hash {
			continue
		}

		s.pulledCount++
		s.syncTaskCh <- &taskData{
			uri:       uri,
			localPath: localPath,
			hash:      hash,
			relPath:   relPath,
		}
	}

	return true
}

func (s *SyncerService) download(task *taskData, downloader *s3manager.Downloader) {
	flog := log.With("uri", task.uri, "localPath", task.localPath)

	// Skip dirs
	if strings.HasSuffix(task.uri, "/") {
		return
	}

	parentDir := filepath.Dir(task.localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		err = os.MkdirAll(parentDir, os.ModePerm)
		if err != nil {
			s.syncTaskErrCh <- fmt.Errorf("can't create directory %s for %s: %v", parentDir, task.localPath, err)
			return
		}
	}

	// Create temp file
	tmpfileName := fmt.Sprintf("%x", md5.Sum([]byte(task.localPath)))
	tmpfilePath := filepath.Join(s.cfg.TempDir, tmpfileName)
	tmpfile, err := os.OpenFile(tmpfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		s.syncTaskErrCh <- fmt.Errorf("can't create temp file: %v", err)
		return
	}
	defer tmpfile.Close()

	s3path, err := parseS3Path(task.uri)
	if err != nil {
		s.syncTaskErrCh <- err
		return
	}

	// Download file
	if _, err := downloader.Download(tmpfile, &s3.GetObjectInput{
		Bucket: aws.String(s3path.bucket),
		Key:    aws.String(s3path.path),
	}); err != nil {
		flog.Errorw("Can't download file", "err", err.Error())
		return
	}

	err = os.Rename(tmpfilePath, task.localPath)
	if err != nil {
		s.syncTaskErrCh <- fmt.Errorf("can't rename temp file %s: %v", task.localPath, err)
		return
	}

	// Update cache
	s.cacheHashLock.Lock()
	s.cacheHash[task.localPath] = task.hash
	s.cacheHashLock.Unlock()
}

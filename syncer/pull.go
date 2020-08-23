package syncer

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Pulls files from S3.
func (s *SyncerService) Pull(threads int) error {
	// Temp dir
	if err := s.createTempDir(); err != nil {
		return err
	}
	defer s.deleteTempDir()

	// Get all files from local dir
	files, err := s.listDir(s.cfg.LocalDir)
	if err != nil {
		return fmt.Errorf("can't list local dir: %v", err)
	}

	// Set all files to delete cache.
	// Files will be removed from cahce by s3ListFunc.
	s.cacheDel = files
	defer func() {
		s.cacheDel = nil
	}()

	s.syncTaskCh = make(chan *taskData, 1000)
	s.syncTaskErrCh = make(chan error, 1000)

	sess, err := session.NewSession(s.getS3cfg())
	if err != nil {
		return fmt.Errorf("can't create new s3 session: %v", err)
	}
	s3svc := s3.New(sess)

	downloader := s3manager.NewDownloaderWithClient(s3svc)

	// Start download workers
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			log.Debugw("Worker started", "id", id)
			for t := range s.syncTaskCh {
				s.download(t, downloader)
			}
			log.Debugw("Worker exited", "id", id)
		}(i)
	}

	var wgErr sync.WaitGroup
	wgErr.Add(1)
	var errMsg error
	go func() {
		defer wgErr.Done()
		for msg := range s.syncTaskErrCh {
			errMsg = msg
			return
		}
	}()

	s.listedCount = 0
	s.pulledCount = 0

	listErr := s3svc.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket: aws.String(s.cfg.s3Bucket),
			Prefix: aws.String(s.cfg.s3Prefix),
		},
		s.procS3,
	)

	close(s.syncTaskCh)
	wg.Wait()
	close(s.syncTaskErrCh)

	if listErr != nil {
		return fmt.Errorf("can't list remote: %v", err)
	}

	wgErr.Wait()

	if err := s.housekeeper(); err != nil {
		return err
	}

	log.Infow("Pull finished", "listed", s.listedCount, "pulled", s.pulledCount)

	return errMsg
}

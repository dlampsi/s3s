package syncer

import (
	"fmt"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var log *zap.SugaredLogger

type SyncerConfig struct {
	// Full path to local directory to sync
	LocalDir string
	// Remote URI to sync files
	RemoteURI string
	// Optional custom S3 endpoint URL.
	S3Endpoint string
	// Temporary directory to store working files
	TempDir string
	// Disable ssl on S3 connections
	DisableSSL bool

	s3Bucket string
	s3Prefix string

	filePulledCnt prometheus.Counter
}

type SyncerService struct {
	cfg *SyncerConfig

	logger *zap.Logger
	reg    *prometheus.Registry

	hashCache       map[string]string
	hashCacheLock   *sync.Mutex
	deleteCache     map[string]bool
	deleteCacheLock *sync.Mutex

	syncTaskCh    chan *syncTask
	syncTaskErrCh chan error

	filesCnt       int
	filesPulledCnt int
}

type SyncerServiceOption func(*SyncerService)

// Specifies logger for service.
func WithLogger(l *zap.Logger) SyncerServiceOption {
	return func(s *SyncerService) { s.logger = l }
}

// Specifies Prometheus registry for service
func WithPrometheusRegistry(reg *prometheus.Registry) SyncerServiceOption {
	return func(s *SyncerService) { s.reg = reg }
}

// Creates new s3s service for operate.
func NewSyncerService(cfg *SyncerConfig, opts ...SyncerServiceOption) (*SyncerService, error) {
	s := &SyncerService{
		cfg:             cfg,
		logger:          zap.NewNop(),
		hashCache:       map[string]string{},
		hashCacheLock:   &sync.Mutex{},
		deleteCache:     map[string]bool{},
		deleteCacheLock: &sync.Mutex{},
	}

	for _, opt := range opts {
		opt(s)
	}

	log = s.logger.Sugar().Named("syncer")
	if s.cfg.TempDir == "" {
		s.cfg.TempDir = "tmp"
	}

	// Check for local dir
	if _, err := os.Stat(s.cfg.LocalDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("can't stat local dir %s, err: %v", s.cfg.LocalDir, err)
	}

	// Parse s3 path
	s3path, err := parseS3Path(s.cfg.RemoteURI)
	if err != nil {
		return nil, fmt.Errorf("can't parse s3 uri: %v", err)
	}
	s.cfg.s3Bucket = s3path.bucket
	s.cfg.s3Prefix = s3path.path

	return s, nil
}

// Pulls files from S3.
func (s *SyncerService) Pull() error {
	log.Info("Pull from S3")

	s.filesCnt = 0
	s.filesPulledCnt = 0

	// Temp dir
	if err := s.createTempDir(); err != nil {
		return err
	}
	defer s.deleteTempDir()

	// Set all files to delete cache.
	// Files will be removed from cahce by s3ListFunc.
	files, err := s.listDir(s.cfg.LocalDir)
	if err != nil {
		return fmt.Errorf("can't list local dir: %v", err)
	}
	log.Debugf("Listed %d files in local dir", len(files))
	s.deleteCache = files
	defer func() {
		s.deleteCache = nil
	}()

	s.syncTaskCh = make(chan *syncTask, 1000)
	s.syncTaskErrCh = make(chan error, 1000)

	if err := s.s3Pull(); err != nil {
		return err
	}

	if err := s.housekeeper(); err != nil {
		return err
	}

	log.Infow("Pull finished", "listed", s.filesCnt, "pulled", s.filesPulledCnt)

	return nil
}

type syncTask struct {
	// File S3 URI
	uri string
	// Local path to file
	localPath string
	// File md5 hash
	hash string
	// Relative file path
	relPath string
}

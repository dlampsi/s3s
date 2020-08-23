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

	cacheHash     map[string]string
	cacheHashLock *sync.Mutex
	cacheDel      map[string]bool
	cacheDelLock  *sync.Mutex

	syncTaskCh    chan *taskData
	syncTaskErrCh chan error

	listedCount int
	pulledCount int
}

type taskData struct {
	// File S3 URI
	uri string
	// Local path to file
	localPath string
	// File md5 hash
	hash string
	// Relative file path
	relPath string
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
		cfg:    cfg,
		logger: zap.NewNop(),

		cacheHash:     map[string]string{},
		cacheHashLock: &sync.Mutex{},
		cacheDel:      map[string]bool{},
		cacheDelLock:  &sync.Mutex{},
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

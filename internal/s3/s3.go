package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	// keyPattern matches: [prefix/]serverID/YYYY/MM/DD/dbname-timestamp.enc
	keyPattern = regexp.MustCompile(`^(?:[^/]+/)*([^/]+)/(\d{4})/(\d{2})/(\d{2})/(.+)-(\d+)\.enc$`)
)

type S3Provider struct {
	client   *s3.Client
	config   Config
	bucket   string
	prefix   string
	serverID string
}

func init() {
	RegisterProvider("s3", NewS3Provider)
	RegisterProvider("r2", NewS3Provider)
	RegisterProvider("b2", NewS3Provider)
	RegisterProvider("wasabi", NewS3Provider)
	RegisterProvider("minio", NewS3Provider)
}

func NewS3Provider(ctx context.Context, cfg Config) (Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid S3 configuration: %w", err)
	}

	var awsOpts []func(*config.LoadOptions) error

	awsOpts = append(awsOpts, config.WithCredentialsProvider(
		credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	))

	// Use auto for R2, cfg.Region or us-east-1 for others
	region := cfg.Region
	if region == "" {
		if cfg.Provider == "r2" {
			region = "auto"
		} else {
			region = "us-east-1"
		}
	}
	awsOpts = append(awsOpts, config.WithRegion(region))

	awsCfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Opts := []func(*s3.Options){}

	if cfg.Endpoint != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.Endpoint)

		// Path-style needed for MinIO, some B2 configurations
		if cfg.PathStyle || cfg.Provider == "minio" {
			s3Opts = append(s3Opts, func(o *s3.Options) {
				o.UsePathStyle = true
			})
		}
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Provider{
		client:   client,
		config:   cfg,
		bucket:   cfg.Bucket,
		prefix:   cfg.Prefix,
		serverID: cfg.ServerID,
	}, nil
}

func (p *S3Provider) GetProviderType() string {
	return p.config.Provider
}

func (p *S3Provider) GetEndpoint() string {
	return p.config.Endpoint
}

func (p *S3Provider) GetBucket() string {
	return p.bucket
}

func (p *S3Provider) generateObjectKey(serverID, databaseName string, timestamp time.Time) string {
	return GenerateObjectKey(p.prefix, serverID, databaseName, timestamp)
}

func (p *S3Provider) parseBackupID(backupID string) (bucket, key string, err error) {
	if strings.HasPrefix(backupID, "s3://") {
		path := strings.TrimPrefix(backupID, "s3://")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid S3 backup ID format: %s", backupID)
		}
		return parts[0], parts[1], nil
	}

	return p.bucket, backupID, nil
}

// extractMetadataFromKey parses serverID, databaseName, and timestamp from object key
// Format: [prefix/]serverID/YYYY/MM/DD/dbname-timestamp.enc
func extractMetadataFromKey(key string) (serverID, dbName string, timestamp time.Time, ok bool) {
	matches := keyPattern.FindStringSubmatch(key)
	if len(matches) != 6 {
		return "", "", time.Time{}, false
	}

	serverID = matches[1]
	dbName = matches[4]

	// Parse timestamp from the key
	if ts, err := strconv.ParseInt(matches[5], 10, 64); err == nil {
		timestamp = time.Unix(ts, 0)
	}

	return serverID, dbName, timestamp, true
}

func (p *S3Provider) Upload(ctx context.Context, serverID, databaseName string, r io.Reader, t time.Time) (*BackupMetadata, error) {
	if serverID == "" {
		serverID = p.serverID
	}
	if serverID == "" {
		return nil, fmt.Errorf("serverID is required")
	}

	key := p.generateObjectKey(serverID, databaseName, t)
	backupID := fmt.Sprintf("s3://%s/%s", p.bucket, key)

	// Get size if r is a ReadSeeker (most common case - *os.File)
	var size int64 = -1
	if seeker, ok := r.(io.ReadSeeker); ok {
		if pos, err := seeker.Seek(0, io.SeekCurrent); err == nil {
			if end, err := seeker.Seek(0, io.SeekEnd); err == nil {
				size = end - pos
				seeker.Seek(pos, io.SeekStart)
			}
		}
	}

	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
		Body:   r,
		Metadata: map[string]string{
			"server-id":     serverID,
			"database-name": databaseName,
			"timestamp":     t.Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	return &BackupMetadata{
		BackupID:        backupID,
		ServerID:        serverID,
		DatabaseName:    databaseName,
		Size:            size,
		CreatedAt:       t,
		RetentionTier:   "standard",
		StorageLocation: backupID,
	}, nil
}

func (p *S3Provider) Download(ctx context.Context, backupID string, w io.Writer) error {
	bucket, key, err := p.parseBackupID(backupID)
	if err != nil {
		return err
	}

	result, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	if _, err := io.Copy(w, result.Body); err != nil {
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	return nil
}

// ListBackups lists backups with O(1) API calls (not N+1)
func (p *S3Provider) ListBackups(ctx context.Context, databaseName string, limit int) ([]BackupMetadata, error) {
	if limit <= 0 {
		limit = 100
	}

	prefix := p.prefix
	if prefix != "" {
		prefix = prefix + "/"
	}
	if p.serverID != "" {
		prefix = prefix + p.serverID + "/"
	}

	result, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects from S3: %w", err)
	}

	var backups []BackupMetadata
	for _, obj := range result.Contents {
		key := aws.ToString(obj.Key)

		// Extract metadata from key path (no additional API call needed)
		serverID, dbName, timestamp, ok := extractMetadataFromKey(key)
		if !ok {
			// Fall back to LastModified if key parsing fails
			serverID = p.serverID
			dbName = ""
			timestamp = aws.ToTime(obj.LastModified)
		}

		if databaseName != "" && dbName != databaseName {
			continue
		}

		backups = append(backups, BackupMetadata{
			BackupID:        fmt.Sprintf("s3://%s/%s", p.bucket, key),
			ServerID:        serverID,
			DatabaseName:    dbName,
			Size:            aws.ToInt64(obj.Size),
			CreatedAt:       timestamp,
			RetentionTier:   "standard",
			StorageLocation: fmt.Sprintf("s3://%s/%s", p.bucket, key),
		})
	}

	return backups, nil
}

func (p *S3Provider) DeleteBackup(ctx context.Context, backupID string) error {
	bucket, key, err := p.parseBackupID(backupID)
	if err != nil {
		return err
	}

	_, err = p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

func (p *S3Provider) HealthCheck(ctx context.Context) error {
	_, err := p.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(p.bucket),
	})
	if err != nil {
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	return nil
}

func (p *S3Provider) GetObjectMetadata(ctx context.Context, backupID string) (*ObjectMetadata, error) {
	bucket, key, err := p.parseBackupID(backupID)
	if err != nil {
		return nil, err
	}

	result, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	metadata := &ObjectMetadata{
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ETag:         aws.ToString(result.ETag),
		Metadata:     make(map[string]string),
	}

	for k, v := range result.Metadata {
		metadata.Metadata[k] = v
	}

	return metadata, nil
}

func (p *S3Provider) ListObjects(ctx context.Context, prefix string) ([]ObjectMetadata, error) {
	fullPrefix := p.prefix
	if fullPrefix != "" {
		fullPrefix = path.Join(fullPrefix, prefix)
	} else {
		fullPrefix = prefix
	}

	result, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.bucket),
		Prefix:  aws.String(fullPrefix),
		MaxKeys: aws.Int32(1000),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var objects []ObjectMetadata
	for _, obj := range result.Contents {
		objects = append(objects, ObjectMetadata{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
		})
	}

	return objects, nil
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check for AWS SDK NotFound error types using errors.As
	var notFoundErr *types.NotFound
	if errors.As(err, &notFoundErr) {
		return true
	}

	var noSuchKeyErr *types.NoSuchKey
	if errors.As(err, &noSuchKeyErr) {
		return true
	}

	// Fallback: check error message for common patterns
	errStr := err.Error()
	return strings.Contains(errStr, "NotFound") ||
		strings.Contains(errStr, "NoSuchKey") ||
		strings.Contains(errStr, "404")
}

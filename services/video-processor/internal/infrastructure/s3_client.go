package infrastructure

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client *s3.Client
}

func NewS3Client() *S3Client {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" && region == "blr1" {
		endpoint = "https://blr1.digitaloceanspaces.com"
	}

	var cfg aws.Config
	var err error

	if endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: region,
			}, nil
		})

		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithEndpointResolverWithOptions(customResolver),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
		)
	}

	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	log.Printf("✅ S3 initialized - Region: %s, Endpoint: %s", region, endpoint)

	return &S3Client{client: client}
}

func (s *S3Client) ListChunks(bucket, prefix string) ([]string, error) {
	var s3Keys []string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			key := *obj.Key
			if key != prefix &&
				!bytes.HasSuffix([]byte(key), []byte("final_recording_")) &&
				bytes.Contains([]byte(key), []byte("chunk_")) {
				s3Keys = append(s3Keys, key)
			}
		}
	}

	// Sort keys by chunk index
	sort.Slice(s3Keys, func(i, j int) bool {
		chunkIndexI := extractChunkIndex(s3Keys[i])
		chunkIndexJ := extractChunkIndex(s3Keys[j])
		return chunkIndexI < chunkIndexJ
	})

	log.Printf("📋 Found %d chunks in S3 folder: %s", len(s3Keys), prefix)
	for i, key := range s3Keys {
		log.Printf("   Chunk %d: %s", i+1, key)
	}

	return s3Keys, nil
}

func (s *S3Client) DownloadChunk(bucket, s3Key, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(s3Key),
		ChecksumMode: "ENABLED",
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, result.Body)
	return err
}

func (s *S3Client) UploadFinalVideo(bucket, s3Key, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(s3Key),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
		ContentType:   aws.String("video/webm"),
		ACL:           "public-read",
	})

	return err
}

func (s *S3Client) CleanupChunks(bucket string, s3Keys []string) {
	log.Printf("🧹 Cleaning up %d chunk files...", len(s3Keys))

	for _, s3Key := range s3Keys {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
		})

		if err != nil {
			log.Printf("Warning: Failed to delete chunk %s: %v", s3Key, err)
		} else {
			log.Printf("🗑️ Deleted chunk: %s", s3Key)
		}
	}

	log.Printf("✅ Cleanup completed")
}

func extractChunkIndex(s3Key string) int {
	parts := bytes.Split([]byte(s3Key), []byte("chunk_"))
	if len(parts) < 2 {
		return 0
	}

	chunkPart := parts[1]
	underscoreIndex := bytes.Index(chunkPart, []byte("_"))
	if underscoreIndex == -1 {
		return 0
	}

	chunkNum := chunkPart[:underscoreIndex]
	var result int
	for _, b := range chunkNum {
		if b >= '0' && b <= '9' {
			result = result*10 + int(b-'0')
		} else {
			break
		}
	}
	return result
}


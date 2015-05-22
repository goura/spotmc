package spotmc

import (
	"bufio"
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/autoscaling"
	"github.com/awslabs/aws-sdk-go/service/s3"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var INSTANCE_ID_URL = "http://169.254.169.254/latest/meta-data/instance-id"

func s3Client() *s3.S3 {
	region := os.Getenv("SPOTMC_AWS_REGION")
	if region == "" {
		region = DEFAULT_REGION
	}

	s3cli := s3.New(&aws.Config{Region: region})
	return s3cli
}

func parseS3URL(s3URLStr string) (bucket, key string, err error) {
	// Parse S3 URL
	u, err := url.Parse(s3URLStr)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != "s3" {
		err := fmt.Errorf("scheme must be 's3': %s", s3URLStr)
		return "", "", err
	}

	bucket = u.Host
	key = strings.TrimPrefix(u.Path, "/")
	return bucket, key, nil
}

func S3Put(s3URLStr, targetPath string) error {
	bucket, key, err := parseS3URL(s3URLStr)
	if err != nil {
		return err
	}

	// Open file
	f, err := os.Open(targetPath)
	if err != nil {
		return err
	}
	fi, err := os.Stat(targetPath)
	if err != nil {
		return err
	}

	// Send request to S3
	s3cli := s3Client()
	req := s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Long(fi.Size()),
	}

	res, err := s3cli.PutObject(&req)
	_ = res
	if err != nil {
		return err
	}

	return nil
}

func S3Get(s3URLStr, targetPath string) error {
	bucket, key, err := parseS3URL(s3URLStr)
	if err != nil {
		return err
	}

	// Send request to S3
	s3cli := s3Client()
	req := s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	res, err := s3cli.GetObject(&req)
	if err != nil {
		return err
	}

	// Open target path
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	// Copy stream
	nBytes, err := io.Copy(w, res.Body)
	_ = nBytes
	if err != nil {
		return err
	}
	w.Flush()

	return nil
}

func autoScalingClient() *autoscaling.AutoScaling {
	region := os.Getenv("SPOTMC_AWS_REGION")
	if region == "" {
		region = DEFAULT_REGION
	}

	asCli := autoscaling.New(&aws.Config{Region: region})
	return asCli
}

func TerminateInstanceInAutoScalingGroup() error {
	// Auto determine myself
	resp, err := http.Get(INSTANCE_ID_URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("could not get instance id")
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	instanceID := string(buf)

	// Terminate the instance and decrement desired capacity
	req := autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceID:                     aws.String(instanceID),
		ShouldDecrementDesiredCapacity: aws.Boolean(true),
	}

	asCli := autoScalingClient()
	_, err = asCli.TerminateInstanceInAutoScalingGroup(&req)
	return err
}

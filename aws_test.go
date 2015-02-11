package spotmc

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/s3"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	uti := time.Now().Unix()
	ut := fmt.Sprintf("%010d", uti)
	bucketName := "spotmc-test-" + ut

	// Create test bucket
	s3cli := s3Client()
	cbConf := s3.CreateBucketConfiguration{
		LocationConstraint: aws.String("us-west-2")}
	cbReq := s3.CreateBucketRequest{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &cbConf,
	}
	_, err := s3cli.CreateBucket(&cbReq)
	if err != nil {
		t.Fatalf("CreateBucket failed(%s): %s", bucketName, err)
	}

	// Put
	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	filePath1 := testDir + "/test1.txt"
	f, err := os.Create(filePath1)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("abcdefg"))
	f.Write([]byte("ABCDEFG"))
	f.Close()

	err = s3Put("s3://"+bucketName+"/foo/bar.txt", filePath1)
	if err != nil {
		t.Fatal("s3Put failed", err)
	}

	// Get
	filePath2 := testDir + "/test2.txt"
	err = s3Get("s3://"+bucketName+"/foo/bar.txt", filePath2)
	if err != nil {
		t.Fatal("s3Get failed", err)
	}

	// Compare
	data, err := ioutil.ReadFile(filePath2)
	if err != nil {
		t.Fatal("ReadFile failed", err)
	}

	if string(data) != "abcdefgABCDEFG" {
		t.Fatalf("Data doesn't match: %s", data)
	}

	// Delete object from S3
	doReq := s3.DeleteObjectRequest{
		Bucket: aws.String(bucketName),
		Key:    aws.String("foo/bar.txt"),
	}
	_, err = s3cli.DeleteObject(&doReq)
	if err != nil {
		t.Fatal("DeleteObject failed")
	}

	// Delete test bucket
	dbReq := s3.DeleteBucketRequest{Bucket: aws.String(bucketName)}
	err = s3cli.DeleteBucket(&dbReq)
	if err != nil {
		t.Fatalf("DeleteBucket failed: %s", bucketName)
	}
}

package spotmc

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/s3"
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
	cbReq := s3.CreateBucketInput{
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

	err = S3Put("s3://"+bucketName+"/foo/bar.txt", filePath1)
	if err != nil {
		t.Fatal("S3Put failed", err)
	}

	// Get
	filePath2 := testDir + "/test2.txt"
	err = S3Get("s3://"+bucketName+"/foo/bar.txt", filePath2)
	if err != nil {
		t.Fatal("S3Get failed", err)
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
	doReq := s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("foo/bar.txt"),
	}
	_, err = s3cli.DeleteObject(&doReq)
	if err != nil {
		t.Fatal("DeleteObject failed")
	}

	// Delete test bucket
	dbReq := s3.DeleteBucketInput{Bucket: aws.String(bucketName)}
	_, err = s3cli.DeleteBucket(&dbReq)
	if err != nil {
		t.Fatalf("DeleteBucket failed: %s", bucketName)
	}
}

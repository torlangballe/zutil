package uaws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zfile"

	// gaws "launchpad.net/goamz/aws" // http://godoc.org/launchpad.net/goamz/aws#Auth
	// gs3 "launchpad.net/goamz/s3"   // http://godoc.org/github.com/danieltoshea/goamz/s3
	"net/http"
	"os"
	"time"
)

// private | public-read | public-read-write | authenticated-read | bucket-owner-read | bucket-owner-full-control

// http://aws.amazon.com/articles/5050 - access
// http://aws.amazon.com/s3/#functionality

const (
	APNortheast  = "ap-northeast-1"
	APSoutheast  = "ap-southeast-1"
	APSoutheast2 = "ap-southeast-2"
	EUWest       = "eu-west-1"
	USEast       = "us-east-1"
	USWest       = "us-west-1"
	USWest2      = "us-west-2"
	SAEast       = "sa-east-1"
	Standard     = USEast
)

var globalAccessKey, globalSecretKey string

func PutFile(s3s *s3.S3, filepath, path, bucketname, mime, permissions, region string) (size int64, err error) {
	size, _ = zfile.GetSize(filepath)
	file, err := os.Open(filepath)
	if err != nil {
		err = errors.WrapN(err, "Error opening file to write to S3", filepath)
		return
	}
	err = PutFromReadSeeker(s3s, file, size, path, bucketname, mime, permissions, region)
	return
}

func ReadFromBucketToFile(sess *session.Session, filepath, s3path, bucket string) (numBytes int64, err error) {
	downloader := s3manager.NewDownloader(sess)

	file, err := os.Create(filepath)
	if err != nil {
		err = errors.Wrap(err, "uaws.ReadFromBucketToFile: os.Create")
		return
	}
	numBytes, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3path),
		})
	if err != nil {
		err = errors.Wrap(err, "uaws.ReadFromBucketToFile: downloader.Download")
		return
	}
	return
}

func PutFromReadSeeker(s3s *s3.S3, r io.ReadSeeker, size int64, path, bucketname, mime, permissions, region string) (err error) {
	params := &s3.PutObjectInput{
		Bucket:        aws.String(bucketname),
		Key:           aws.String(path),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(mime),
	}
	_, err = s3s.PutObject(params)
	if err != nil {
		err = errors.Wrap(err, "svc.PutObject")
		return
	}
	//	fmt.Printf("PutFromReadSeeker response %s\n", awsutil.StringValue(resp))
	return
}

func ListObjectsInBucket(s3s *s3.S3, region, bucket, marker string, got func(path string, modified time.Time, size int64)) (err error) {
	input := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	}
	if marker != "" {
		input.Marker = aws.String(marker)
	}
	output, err := s3s.ListObjects(input)
	if err != nil {
		err = errors.Wrap(err, "s3s.ListObjectsPages")
		return
	}
	for _, c := range output.Contents {
		got(*c.Key, *c.LastModified, *c.Size)
	}
	return
}

func DeleteS3Object(s3s *s3.S3, region, bucket, path string) (err error) {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}
	_, err = s3s.DeleteObject(input)
	return
}

func GetS3FileSize(s3s *s3.S3, region, bucket, path string) (size int64, found bool, err error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	head, err := s3s.HeadObject(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "NotFound" {
				err = nil
			}
		}
		return
	}
	found = true
	if head.ContentLength == nil {
		err = errors.New("No Content Length")
		return
	} else {
		size = *head.ContentLength
	}
	return
}

func PresignBucketItemForDownload(s3s *s3.S3, region, bucket, key string, liveMins int) (signedUrl string, err error) {
	req, _ := s3s.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	signedUrl, err = req.Presign(15 * time.Minute)
	if err != nil {
		fmt.Println("Failed to sign request", err)
		return
	}
	return
}

// takes a region, bucketm key, md5, mins to live, and creates url for uploading it.
// Note you have to set the cors info in the permissions of the bucket to do this from js.
// <AllowedOrigin>*</AllowedOrigin> - from any domain
// <AllowedMethod>GET</AllowedMethod>
// <AllowedMethod>PUT</AllowedMethod> - make sure we can put etc
// <AllowedHeader>*</AllowedHeader> - make sure we can send a bunch of headers
func PresignForUploadToBucketItem(s3s *s3.S3, region, bucket, key string, liveMins int) (surl string, downloadUrl string, err error) {
	if err != nil {
		fmt.Println("PresignForUploadToBucketItem: error getting session:", err)
		return
	}
	r, _ := s3s.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		//		ACL:           aws.String("public-read-write"),
		//		ContentType:   aws.String(contentType),
		//		ContentLength: &contentLength,
	})
	//	var signedHeaders http.Header //	r.HTTPRequest.Header.Set("Content-MD5", md5checksum)
	surl, _, err = r.PresignRequest(time.Duration(liveMins) * time.Minute)
	if err != nil {
		fmt.Println("error presigning request:", err)
		return
	}
	downloadUrl = zstr.HeadUntilString(surl, "?")
	//	fmt.Println("PresignForUploadToBucketItem: headers:", signedHeaders)
	return
}

func uploadToBucketFromBody(sess *session.Session, fileKey, bucket, region string, body io.Reader) (err error) {
	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.PartSize = 20 << 20 // 20MB
	})
	ctx, _ := context.WithCancel(context.Background())
	_, err = uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fileKey),
		Body:   body,
	})
	return
}

func UploadToBucketFromRequest(sess *session.Session, fileKey, bucket, region string, w http.ResponseWriter, req *http.Request) (err error) {
	return uploadToBucketFromBody(sess, fileKey, bucket, region, req.Body)
}

func UploadToBucketFromFile(sess *session.Session, fileKey, bucket, region string, filePath string) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	return uploadToBucketFromBody(sess, fileKey, bucket, region, file)
}

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

type StatementEntry struct {
	Sid      string
	Effect   string
	Action   []string
	Resource string
}

const DefaultPolicyVersion = "2012-10-17"

func CreatePolicy(iamClient *iam.IAM, name string, doc interface{}) (output *iam.CreatePolicyOutput, err error) {
	policyInput := &iam.CreatePolicyInput{}
	policyInput.SetPolicyName(name)
	docBytes, err := json.Marshal(doc)
	if err != nil {
		err = errors.Wrap(err, "uaws.CreatePolicy marshal")
		return
	}
	policyInput.SetPolicyDocument(string(docBytes))
	output, err = iamClient.CreatePolicy(policyInput)
	if err != nil {
		err = errors.Wrap(err, "uaws.CreatePolicy iam.CreatePolicy")
		return
	}
	return
}

func GetPolicy(iamClient *iam.IAM, arn string) (*iam.Policy, error) {
	result, err := iamClient.GetPolicy(&iam.GetPolicyInput{
		PolicyArn: &arn,
	})
	if err != nil {
		return nil, err
	}
	return result.Policy, nil
}

func AttachPolicyToUser(iamClient *iam.IAM, userName, policyArn string) (err error) {
	input := &iam.AttachUserPolicyInput{}
	input.SetUserName(userName)
	input.SetPolicyArn(policyArn)
	_, err = iamClient.AttachUserPolicy(input)
	if err != nil {
		err = errors.Wrap(err, "uaws.AttachPolicyToUser iam.AttachUserPolicy")
		return
	}
	return
}
func CreateUserAccessKeys(iamClient *iam.IAM, name string) (key, secret string, err error) {
	createInput := &iam.CreateAccessKeyInput{}
	createInput.SetUserName(name)
	output, err := iamClient.CreateAccessKey(createInput)
	if err != nil {
		err = errors.Wrap(err, "uaws.CreateUserAccessKeys iam.CreateAccessKey")
		return
	}
	key = aws.StringValue(output.AccessKey.AccessKeyId)
	secret = aws.StringValue(output.AccessKey.SecretAccessKey)
	return
}

func CreateSession(region, accesskey, secretKey, token string) (sess *session.Session, err error) {
	if token == "" {
		token = "session-token"
	}
	a := accesskey
	creds := credentials.NewStaticCredentials(a, secretKey, token)
	creds.Get()
	sess, err = session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})
	// sess, err = session.NewSession(&aws.Config{
	// 	Region:      aws.String(region),
	// 	Credentials: creds,
	// })
	return
}

// DynamoDB helpers:

var NoDynamoDbRow = errors.New("no row")

func DynamoSession(awsSess *session.Session) *dynamodb.DynamoDB {
	return dynamodb.New(awsSess)
}

func LocalDynamoSession() *dynamodb.DynamoDB {
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(Standard),
		Endpoint: aws.String("http://localhost:8000")})
	if err != nil {
		fmt.Println("LocalDynamoSession err:", err)
	}
	return dynamodb.New(sess)
}

func PutDynamoItem(dsess *dynamodb.DynamoDB, item interface{}, table string) (err error) {
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}
	//	fmt.Println("UAWS.PutDynamoItem:", av)
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(table),
	}

	_, err = dsess.PutItem(input)

	if err != nil {
		return err
	}
	return
}

func GetDynamoItem(dsess *dynamodb.DynamoDB, item interface{}, table string, keys map[string]*dynamodb.AttributeValue) (got bool, err error) {
	result, err := dsess.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key:       keys,
	})

	if err != nil {
		return
	}

	if result == nil || result.Item == nil || len(result.Item) == 0 {
		return
	}
	got = true

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)

	if err != nil {
		return
	}
	return
}

func GetDynamoItemFromStringKey(dsess *dynamodb.DynamoDB, item interface{}, table, key, value string) (got bool, err error) {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/dynamodb/#AttributeValue
	keys := map[string]*dynamodb.AttributeValue{
		key: {
			S: aws.String(value),
		},
	}
	return GetDynamoItem(dsess, item, table, keys)
}

func DeleteDynamoItem(dsess *dynamodb.DynamoDB, table string, keys map[string]*dynamodb.AttributeValue) (err error) {
	input := &dynamodb.DeleteItemInput{
		Key:       keys,
		TableName: aws.String(table),
	}
	_, err = dsess.DeleteItem(input)
	return
}

func DeleteDynamoItemWithStringKey(dsess *dynamodb.DynamoDB, table, key string) (err error) {
	keys := map[string]*dynamodb.AttributeValue{
		key: {
			S: aws.String(key),
		},
	}
	return DeleteDynamoItem(dsess, table, keys)
}

func CreateDynamoTable(dsess *dynamodb.DynamoDB, name string, attributes map[string]string) (err error) {
	var ttlField string
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{},
		KeySchema:            []*dynamodb.KeySchemaElement{},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(2),
			WriteCapacityUnits: aws.Int64(2),
		},
		TableName: aws.String(name),
	}
	for k, v := range attributes {
		var a = new(dynamodb.AttributeDefinition)
		var e = new(dynamodb.KeySchemaElement)
		a.AttributeName = aws.String(k)
		e.AttributeName = aws.String(k)
		var atype, ktype string
		if zstr.SplitN(v, ":", &atype, &ktype) {
			if ktype == "EXPIRY" {
				ttlField = k
				continue
			}
		} else {
			atype = v
			ktype = ""

		}
		a.AttributeType = aws.String(atype)
		input.AttributeDefinitions = append(input.AttributeDefinitions, a)
		if ktype != "" {
			e.KeyType = aws.String(ktype)
			input.KeySchema = append(input.KeySchema, e)
		}
	}
	_, err = dsess.CreateTable(input)
	if err != nil {
		if ttlField != "" {
			var ttlInput dynamodb.UpdateTimeToLiveInput
			ttlInput.SetTableName(name)
			var spec dynamodb.TimeToLiveSpecification
			spec.SetAttributeName(ttlField)
			spec.SetEnabled(true)
			ttlInput.SetTimeToLiveSpecification(&spec)
			dsess.UpdateTimeToLive(&ttlInput)
		}
	}
	return
}

func GetDyanamoTables(dsess *dynamodb.DynamoDB) (tables []string) {
	result, err := dsess.ListTables(&dynamodb.ListTablesInput{})
	if err != nil {
		return
	}
	for _, n := range result.TableNames {
		tables = append(tables, *n)
	}
	return
}

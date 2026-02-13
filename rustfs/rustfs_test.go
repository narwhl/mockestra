package rustfs_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	container "github.com/narwhl/mockestra/rustfs"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestRustFSModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"rustfs_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("rustfs-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"rustfs"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client, err := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4("rustfsadmin", "rustfsadmin", ""),
				Secure: false,
			})
			if err != nil {
				t.Errorf("failed to create rustfs client: %v", err)
			}
			_, err = client.ListBuckets(t.Context())
			if err != nil {
				t.Errorf("failed to list buckets: %v", err)
				return
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestRustFSModule_WithBucket(t *testing.T) {
	expectedBucket := "test-bucket"
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"rustfs_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("rustfs-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithBucket(expectedBucket),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"rustfs"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client, err := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4("rustfsadmin", "rustfsadmin", ""),
				Secure: false,
			})
			if err != nil {
				t.Errorf("failed to create rustfs client: %v", err)
			}
			buckets, err := client.ListBuckets(t.Context())
			if err != nil {
				t.Errorf("failed to list buckets: %v", err)
				return
			}
			found := false
			for _, bucket := range buckets {
				if bucket.Name == expectedBucket {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected bucket '%s' to be created, got: %v", expectedBucket, buckets)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestRustFSModule_WithPolicyAndExpiration(t *testing.T) {
	bucketName := "policy-test-bucket"
	prefix := "allowed-prefix/"
	expirationDuration := 1 * time.Hour

	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"rustfs_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("rustfs-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithBucket(bucketName),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"rustfs"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}

			adminClient, err := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4("rustfsadmin", "rustfsadmin", ""),
				Secure: false,
			})
			if err != nil {
				t.Fatalf("failed to create admin client: %v", err)
			}

			var otherBucket = "other-bucket"
			err = adminClient.MakeBucket(t.Context(), otherBucket, minio.MakeBucketOptions{})
			if err != nil {
				t.Fatalf("failed to create other bucket: %v", err)
			}

			policyJSON := fmt.Sprintf(`{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"],
						"Resource": ["arn:aws:s3:::%s", "arn:aws:s3:::%s/%s*"]
					}
				]
			}`, bucketName, bucketName, prefix)

			stsEndpoint := "http://" + endpoint + "/sts"
			assumeRoleOpts := credentials.STSAssumeRoleOptions{
				AccessKey:       "rustfsadmin",
				SecretKey:       "rustfsadmin",
				DurationSeconds: int(expirationDuration.Seconds()),
				Policy:          policyJSON,
			}

			assumeRoleCreds, err := credentials.NewSTSAssumeRole(stsEndpoint, assumeRoleOpts)
			if err != nil {
				t.Fatalf("failed to create assume role credentials: %v", err)
			}

			tempCreds, err := assumeRoleCreds.Get()
			if err != nil {
				t.Fatalf("failed to get temporary credentials: %v", err)
			}

			if tempCreds.Expiration.IsZero() {
				t.Fatalf("expected credentials to have expiration time set")
			}

			minExpected := time.Now().Add(expirationDuration - 10*time.Second)
			if tempCreds.Expiration.Before(minExpected) {
				t.Fatalf("expected expiration to be at least %v, got %v", minExpected, tempCreds.Expiration)
			}
			t.Logf("Temporary credentials - AccessKey: %s, SecretKey: %s, Expiration: %v", tempCreds.AccessKeyID, tempCreds.SecretAccessKey, tempCreds.Expiration)

			restrictedClient, err := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4(tempCreds.AccessKeyID, tempCreds.SecretAccessKey, tempCreds.SessionToken),
				Secure: false,
			})
			if err != nil {
				t.Fatalf("failed to create restricted client: %v", err)
			}

			testObjectKey := prefix + "test-object.txt"
			testContent := []byte("test content for policy-based access")
			_, err = restrictedClient.PutObject(t.Context(), bucketName, testObjectKey, bytes.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{})
			if err != nil {
				t.Fatalf("failed to upload object with restricted credentials: %v", err)
			}

			deniedObjectKey := "other-prefix/test-object.txt"
			_, err = restrictedClient.PutObject(t.Context(), bucketName, deniedObjectKey, bytes.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{})
			if err == nil {
				t.Fatalf("expected upload to be denied for prefix '%s', but it succeeded", deniedObjectKey)
			}
			t.Logf("Correctly denied access to prefix '%s': %v", deniedObjectKey, err)

			objectKeyWithoutPrefix := "another-object.txt"
			_, err = restrictedClient.PutObject(t.Context(), bucketName, objectKeyWithoutPrefix, bytes.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{})
			if err == nil {
				t.Fatalf("expected upload to be denied for object without prefix '%s', but it succeeded", objectKeyWithoutPrefix)
			}
			t.Logf("Correctly denied access to object without prefix: %v", err)

			otherBucketKey := prefix + "test.txt"
			_, err = restrictedClient.PutObject(t.Context(), otherBucket, otherBucketKey, bytes.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{})
			if err == nil {
				t.Fatalf("expected upload to be denied for bucket '%s', but it succeeded", otherBucket)
			}
			t.Logf("Correctly denied access to other bucket '%s': %v", otherBucket, err)
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

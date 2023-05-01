/*
Copyright (C) 2021-2023, Kubefirst

This program is licensed under MIT.
See the LICENSE file for more details.
*/
package objectStorage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kubefirst/kubefirst-api/internal/types"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
)

// PutBucketObject
func PutBucketObject(cr *types.StateStoreCredentials, d *types.StateStoreDetails, obj *types.PushBucketObject) error {
	ctx := context.Background()

	// Initialize minio client object.
	minioClient, err := minio.New(d.Hostname, &minio.Options{
		Creds:  credentials.NewStaticV4(cr.AccessKeyID, cr.SecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("error initializing minio client: %s", err)
	}

	object, err := os.Open(obj.LocalFilePath)
	if err != nil {
		return err
	}
	defer object.Close()

	objectStat, err := object.Stat()
	if err != nil {
		return err
	}

	n, err := minioClient.PutObject(ctx, d.Name, obj.RemoteFilePath, object, objectStat.Size(), minio.PutObjectOptions{ContentType: obj.ContentType})
	if err != nil {
		return err
	}
	log.Info("uploaded", obj.LocalFilePath, " of size: ", n, "successfully")

	return nil
}

// PutClusterObject exports a cluster definition as json and places it in the target object storage bucket
func PutClusterObject(cr *types.StateStoreCredentials, d *types.StateStoreDetails, obj *types.PushBucketObject) error {
	ctx := context.Background()

	// Initialize minio client
	minioClient, err := minio.New(d.Hostname, &minio.Options{
		Creds:  credentials.NewStaticV4(cr.AccessKeyID, cr.SecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("error initializing minio client: %s", err)
	}

	// Reference for cluster object output file
	object, err := os.Open(obj.LocalFilePath)
	if err != nil {
		return fmt.Errorf("error during object local copy file lookup: %s", err)
	}
	defer object.Close()

	objectStat, err := object.Stat()
	if err != nil {
		return fmt.Errorf("error during object stat: %s", err)
	}

	// Put
	_, err = minioClient.PutObject(
		ctx,
		d.Name,
		obj.RemoteFilePath,
		object,
		objectStat.Size(),
		minio.PutObjectOptions{ContentType: obj.ContentType},
	)
	if err != nil {
		return fmt.Errorf("error during object put: %s", err)
	}
	log.Infof("uploaded cluster object %s to state store bucket %s successfully", obj.LocalFilePath, d.Name)

	return nil
}

// GetClusterObject exports a cluster definition as json and places it in the target object storage bucket
func GetClusterObject(cr *types.StateStoreCredentials, d *types.StateStoreDetails, clusterName string, localFilePath string, remoteFilePath string) error {
	ctx := context.Background()

	// Initialize minio client
	minioClient, err := minio.New(d.Hostname, &minio.Options{
		Creds:  credentials.NewStaticV4(cr.AccessKeyID, cr.SecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("error initializing minio client: %s", err)
	}

	// Get object from bucket
	reader, err := minioClient.GetObject(ctx, d.Name, remoteFilePath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving cluster object from bucket: %s", err)
	}
	defer reader.Close()

	// Write object to local file
	localFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("error during object local copy file create: %s", err)
	}
	defer localFile.Close()

	stat, err := reader.Stat()
	if err != nil {
		return fmt.Errorf("error during object stat: %s", err)
	}
	if _, err := io.CopyN(localFile, reader, stat.Size); err != nil {
		return fmt.Errorf("error during object copy: %s", err)
	}

	return nil
}
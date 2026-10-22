// Package azureblob uploads files to an Azure Blob Storage container with
// shared-key authentication. The container is expected to allow public blob
// read access, so the URL it returns for an uploaded blob is permanent and can
// be stored as-is.
package azureblob

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

// Credential keys stored in ecom_storage_disk_credentials for an Azure disk.
const (
	KeyAccountName   = "account_name"
	KeyAccountKey    = "account_key"
	KeyContainerName = "container_name"
)

var ErrMissingCredentials = errors.New("azure blob credentials are incomplete: account_name, account_key and container_name are required")

type Client struct {
	client    *azblob.Client
	container string
}

// NewClient builds a client from a disk's decrypted credentials (keyed by the
// Key* constants).
func NewClient(credentials map[string]string) (*Client, error) {
	accountName := strings.TrimSpace(credentials[KeyAccountName])
	accountKey := strings.TrimSpace(credentials[KeyAccountKey])
	container := strings.TrimSpace(credentials[KeyContainerName])
	if accountName == "" || accountKey == "" || container == "" {
		return nil, ErrMissingCredentials
	}

	sharedKey, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("invalid azure storage account key: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(fmt.Sprintf("https://%s.blob.core.windows.net/", accountName), sharedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating azure blob client: %w", err)
	}

	return &Client{client: client, container: container}, nil
}

// Upload writes data to blobName (overwriting any blob with that name) with the
// given content type and returns the blob's public URL.
func (c *Client) Upload(ctx context.Context, blobName, contentType string, data []byte) (string, error) {
	if _, err := c.client.UploadBuffer(ctx, c.container, blobName, data, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: &contentType},
	}); err != nil {
		return "", fmt.Errorf("error uploading blob %q to container %q: %w", blobName, c.container, err)
	}

	return c.BlobURL(blobName), nil
}

// BlobURL is the public URL blobName has (or will have) in the container, with
// no network call.
func (c *Client) BlobURL(blobName string) string {
	return c.client.ServiceClient().NewContainerClient(c.container).NewBlobClient(blobName).URL()
}

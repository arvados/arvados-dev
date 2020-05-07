// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/arvados/arvados-dev/compute-image-cleaner/config"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"

	"code.cloudfoundry.org/bytefmt"
)

type blob struct {
	name              string
	created           time.Time
	contentLength     int64
	deletionCandidate bool
}

func prepAzBlob(storageKey string, account string, container string) (p pipeline.Pipeline, containerURL azblob.ContainerURL) {
	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(account, storageKey)
	if err != nil {
		log.Fatal("Invalid credentials with error: " + err.Error())
	}
	p = azblob.NewPipeline(credential, azblob.PipelineOptions{})
	// From the Azure portal, get your storage account blob service URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", account, container))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL = azblob.NewContainerURL(*URL, p)

	return
}

func loadBlobs(p pipeline.Pipeline, containerURL azblob.ContainerURL) (blobs []blob, blobNames map[string]*blob) {
	blobNames = make(map[string]*blob)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for marker := (azblob.Marker{}); marker.NotDone(); {
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			log.Fatal("Error getting blob list: " + err.Error())
		}

		// ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			blobs = append(blobs, blob{name: blobInfo.Name, created: *blobInfo.Properties.CreationTime, contentLength: *blobInfo.Properties.ContentLength})
			blobNames[blobInfo.Name] = &blobs[len(blobs)-1]
		}
	}
	sort.Slice(blobs, func(i, j int) bool { return blobs[i].created.After(blobs[j].created) })

	return
}

func weedBlobs(blobs []blob, blobNames map[string]*blob, containerURL azblob.ContainerURL, account string, container string, doIt bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pairedFileName string
	skipCount := 10
	t := time.Now()
	thirtyDaysAgo := t.AddDate(0, 0, -30)

	// e.g. su92l-compute-osDisk.866eb426-8d1e-45ad-91be-2bb55b5a8147.vhd
	vhd := regexp.MustCompile(`^(.*)-compute-osDisk\.(.*)\.vhd$`)
	// e.g. su92l-compute-vmTemplate.866eb426-8d1e-45ad-91be-2bb55b5a8147.json
	json := regexp.MustCompile(`^(.*)-compute-vmTemplate\.(.*)\.json$`)

	for i, blob := range blobs {
		matches := vhd.FindStringSubmatch(blob.name)
		if len(matches) > 1 {
			// osDisk image file
			pairedFileName = matches[1] + "-compute-vmTemplate." + matches[2] + ".json"
		} else {
			matches := json.FindStringSubmatch(blob.name)
			if len(matches) > 1 {
				// vmTemplate file
				pairedFileName = matches[1] + "-compute-osDisk." + matches[2] + ".vhd"
			} else {
				log.Println("Skipping blob because name does not match a known file name pattern:", blob.name, " ", blob.created)
				continue
			}
		}
		if blob.created.After(thirtyDaysAgo) {
			log.Println("Skipping blob because it was created less than 30 days ago:", blob.name, " ", blob.created)
			skipCount = skipCount - 1
			continue
		}
		if skipCount > 0 {
			log.Println("Skipping blob because it's in the top 10 most recent list:", blob.name, " ", blob.created)
			skipCount = skipCount - 1
			continue
		}
		if _, ok := blobNames[pairedFileName]; !ok {
			log.Println("Warning: paired file", pairedFileName, "not found for blob", blob.name, " ", blob.created)
		}
		blobs[i].deletionCandidate = true
	}

	var reclaimedSpace, otherSpace int64

	for _, blob := range blobs {
		if blob.deletionCandidate {
			log.Println("Candidate for deletion:", blob.name, " ", blob.created)
			reclaimedSpace = reclaimedSpace + blob.contentLength

			if doIt {
				log.Println("Deleting:", blob.name, " ", blob.created)
				blockBlobURL := containerURL.NewBlockBlobURL(blob.name)
				result, err := blockBlobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
				if err != nil {
					log.Println(result)
					log.Fatal("Error deleting blob: ", err.Error(), "\n", result)
				}
			}
		} else {
			otherSpace = otherSpace + blob.contentLength
		}
	}

	if doIt {
		log.Println("Reclaimed", bytefmt.ByteSize(uint64(reclaimedSpace)), "or", reclaimedSpace, "bytes.")
	} else {
		log.Println("Deletion not requested. Able to reclaim", bytefmt.ByteSize(uint64(reclaimedSpace)), "or", reclaimedSpace, "bytes.")
	}
	log.Println("Kept", bytefmt.ByteSize(uint64(otherSpace)), "or", otherSpace, "bytes.")

}

func loadStorageAccountKey(resourceGroup string, account string) (key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storageClient := getStorageAccountsClient()
	keys, err := storageClient.ListKeys(ctx, resourceGroup, account)
	if err != nil {
		log.Fatal("Error getting storage account key:", err.Error())
	}

	key = *(*keys.Keys)[0].Value

	return
}

func validateInputs() (resourceGroup string, account string, container string, doIt bool) {
	err := config.ParseEnvironment()
	if err != nil {
		log.Fatal("Unable to parse environment")
	}

	if config.ClientID() == "" || config.ClientSecret() == "" || config.TenantID() == "" || config.SubscriptionID() == "" {
		log.Fatal("Please make sure the environment variables AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID are set")
	}

	flags := flag.NewFlagSet("compute-image-cleaner", flag.ExitOnError)
	flags.StringVar(&resourceGroup, "resourceGroup", "", "Name of the Azure resource group")
	flags.StringVar(&account, "account", "", "Name of the Azure storage account")
	flags.StringVar(&container, "container", "", "Name of the container in the Azure storage account")
	flags.BoolVar(&doIt, "delete", false, "Delete blobs that meet criteria (default: false)")
	flags.Usage = func() { usage(flags) }
	err = flags.Parse(os.Args[1:])

	if err != nil || resourceGroup == "" || account == "" || container == "" {
		usage(flags)
		os.Exit(1)
	}

	return
}

func main() {
	resourceGroup, account, container, doIt := validateInputs()
	storageKey := loadStorageAccountKey(resourceGroup, account)
	p, containerURL := prepAzBlob(storageKey, account, container)

	blobs, blobNames := loadBlobs(p, containerURL)
	weedBlobs(blobs, blobNames, containerURL, account, container, doIt)
}

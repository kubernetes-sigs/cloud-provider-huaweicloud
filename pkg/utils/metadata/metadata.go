/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"k8s.io/utils/mount"
)

const (
	defaultMetadataVersion = "latest"
	metadataURLTemplate    = "http://169.254.169.254/openstack/%s/meta_data.json"

	// MetadataID is used as an identifier on the metadata search order configuration.
	MetadataID = "metadataService"

	// Config drive is defined as an iso9660 or vfat (deprecated) drive
	// with the "config-2" label.
	//https://docs.openstack.org/nova/latest/user/config-drive.html
	configDriveLabel        = "config-2"
	configDrivePathTemplate = "openstack/%s/meta_data.json"

	// ConfigDriveID is used as an identifier on the metadata search order configuration.
	ConfigDriveID = "configDrive"
)

// ErrBadMetadata is used to indicate a problem parsing data from metadata server
var ErrBadMetadata = errors.New("invalid HuaweiCloud metadata, got empty uuid")

// Metadata is fixed for the current host, so cache the value process-wide
var metadataCache *Metadata

// Metadata has the information fetched from HuaweiCloud metadata service or
// config drives. Assumes the "latest" meta_data.json format.
type Metadata struct {
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	AvailabilityZone string `json:"availability_zone"`
	RegionID         string `json:"region_id"`
	// .. and other fields we don't care about.  Expand as necessary.
}

// IMetadata implements GetInstanceID & GetAvailabilityZone
type IMetadata interface {
	GetInstanceID() (string, error)
	GetAvailabilityZone() (string, error)
}

// parseMetadata reads JSON from HuaweiCloud metadata server and parses
// instance ID out of it.
func parseMetadata(r io.Reader) (*Metadata, error) {
	var metadata Metadata
	jsonData := json.NewDecoder(r)
	if err := jsonData.Decode(&metadata); err != nil {
		return nil, err
	}

	if metadata.UUID == "" {
		return nil, ErrBadMetadata
	}

	return &metadata, nil
}

func getMetadataURL(metadataVersion string) string {
	return fmt.Sprintf(metadataURLTemplate, metadataVersion)
}

func getConfigDrivePath(metadataVersion string) string {
	return fmt.Sprintf(configDrivePathTemplate, metadataVersion)
}

func getFromConfigDrive(metadataVersion string) (*Metadata, error) {
	// Try to read instance UUID from config drive.
	dev := "/dev/disk/by-label/" + configDriveLabel
	if _, err := os.Stat(dev); os.IsNotExist(err) {
		out, err := exec.New().Command(
			"blkid", "-l",
			"-t", "LABEL="+configDriveLabel,
			"-o", "device",
		).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("unable to run blkid: %v", err)
		}
		dev = strings.TrimSpace(string(out))
	}
	mntdir, err := os.MkdirTemp("", "configdrive")
	if err != nil {
		return nil, err
	}
	defer func() {
		err := os.Remove(mntdir)
		if err != nil {
			klog.Warningf("error deleting tmp path: %s", mntdir)
		}
	}()

	klog.V(4).Infof("Attempting to mount config drive %s on %s", dev, mntdir)

	mounter := getBaseMounter()
	err = mounter.Mount(dev, mntdir, "iso9660", []string{"ro"})
	if err != nil {
		err = mounter.Mount(dev, mntdir, "vfat", []string{"ro"})
	}
	if err != nil {
		return nil, fmt.Errorf("error mounting config drive %s: %v", dev, err)
	}
	defer func() {
		err := mounter.Unmount(mntdir)
		if err != nil {
			klog.Errorf("error unmount, path: %s, error: %s", mntdir, err)
		}
	}()

	klog.V(4).Infof("Config drive mounted on %s", mntdir)

	configDrivePath := getConfigDrivePath(metadataVersion)
	f, err := os.Open(
		filepath.Join(mntdir, configDrivePath))
	if err != nil {
		return nil, fmt.Errorf("error reading %s on config drive: %v", configDrivePath, err)
	}
	defer f.Close()

	return parseMetadata(f)
}

func getBaseMounter() *mount.SafeFormatAndMount {
	nMounter := mount.New("")
	nExec := exec.New()
	return &mount.SafeFormatAndMount{
		Interface: nMounter,
		Exec:      nExec,
	}
}

func getFromMetadataService(metadataVersion string) (*Metadata, error) {
	// Try to get JSON from metadata server.
	url := getMetadataURL(metadataVersion)
	klog.V(4).Infof("Attempting to fetch metadata from %s", url)
	resp, err := http.Get(url) //nolint: gosec
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected status code when reading metadata from %s: %s", url, resp.Status)
		return nil, err
	}

	return parseMetadata(resp.Body)
}

// Get retrieves metadata from either config drive or metadata service.
// Search order depends on the order set in config file.
func Get(order string) (*Metadata, error) {
	if metadataCache == nil {
		var md *Metadata
		var err error

		elements := strings.Split(order, ",")
		for _, id := range elements {
			id = strings.TrimSpace(id)
			switch id {
			case ConfigDriveID:
				md, err = getFromConfigDrive(defaultMetadataVersion)
			case MetadataID:
				md, err = getFromMetadataService(defaultMetadataVersion)
			default:
				err = fmt.Errorf("%s is not a valid metadata search order option. Supported options are %s and %s", id, ConfigDriveID, MetadataID)
			}

			if err == nil {
				break
			}
		}

		if err != nil {
			return nil, err
		}
		metadataCache = md
	}
	return metadataCache, nil
}

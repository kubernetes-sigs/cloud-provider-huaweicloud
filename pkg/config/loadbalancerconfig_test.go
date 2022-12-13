package config

import (
	"strconv"
	"testing"
)

func TestLoadELBConfigBasic(t *testing.T) {
	const (
		networkID   = "1234"
		subnetID    = "5678"
		keepEip     = true
		healthCheck = "on"

		publicNetworkName   = "public-network-name"
		internalNetworkName = "internal-network-name"

		lbProvider = "vlb"
		lbMethod   = "ROUND_ROBIN"

		searchOrder = "metadataService,configDrive"
	)
	str := `
[LoadBalancerOptions]
network-id=` + networkID + `
subnet-id=` + subnetID + `
keep-eip=` + strconv.FormatBool(keepEip) + `
health-check=` + healthCheck + `

[NetworkingOptions]
public-network-name=` + publicNetworkName + `1
public-network-name=` + publicNetworkName + `2
internal-network-name=` + internalNetworkName + `1
internal-network-name=` + internalNetworkName + `2
`
	cfg, err := LoadELBConfig(str)
	if err != nil {
		t.Fatalf("error loadbabalancer config: %s", err)
	}

	if cfg.LoadBalancerOpts.NetworkID != networkID {
		t.Fatalf("NetworkID, expected: %v, got: %v", networkID, cfg.LoadBalancerOpts.NetworkID)
	}
	if cfg.LoadBalancerOpts.SubnetID != subnetID {
		t.Fatalf("SubnetID, expected: %v, got: %v", subnetID, cfg.LoadBalancerOpts.SubnetID)
	}
	if cfg.LoadBalancerOpts.SessionAffinityMode != lbMethod {
		t.Fatalf("SessionAffinityMode, expected: %v, got: %v", lbMethod, cfg.LoadBalancerOpts.SessionAffinityMode)
	}
	if cfg.LoadBalancerOpts.LBProvider != lbProvider {
		t.Fatalf("LBProvider, expected: %v, got: %v", lbProvider, cfg.LoadBalancerOpts.LBProvider)
	}
	if cfg.LoadBalancerOpts.KeepEIP != keepEip {
		t.Fatalf("KeepEIP, expected: %v, got: %v", keepEip, cfg.LoadBalancerOpts.KeepEIP)
	}

	publicNetworkNames := cfg.NetworkingOpts.PublicNetworkName
	if publicNetworkNames[0] != publicNetworkName+"1" || publicNetworkNames[1] != publicNetworkName+"2" {
		t.Fatalf("PublicNetworkName, expected: %v, got: %v", publicNetworkName, cfg.LoadBalancerOpts.NetworkID)
	}

	internalNetworkNames := cfg.NetworkingOpts.InternalNetworkName
	if internalNetworkNames[0] != internalNetworkName+"1" || internalNetworkNames[1] != internalNetworkName+"2" {
		t.Fatalf("InternalNetworkName, expected: %v, got: %v", internalNetworkName, internalNetworkNames)
	}

	if cfg.MetadataOpts.SearchOrder != searchOrder {
		t.Fatalf("SearchOrder, expected: %v, got: %v", searchOrder, cfg.MetadataOpts.SearchOrder)
	}
}

func TestLoadELBConfigAll(t *testing.T) {
	const (
		networkID         = "1234"
		subnetID          = "5678"
		lbMethod          = "SOURCE_IP"
		lbProvider        = "vlb"
		keepEip           = true
		healthCheck       = "off"
		HealthCheckOption = "{}"

		publicNetworkName   = "public-network-name"
		internalNetworkName = "internal-network-name"

		searchOrder            = "configDrive,metadataService"
		metadataRequestTimeout = "10s"
	)
	str := `
[LoadBalancerOptions]
network-id=` + networkID + `
subnet-id=` + subnetID + `
session-affinity-mode=` + lbMethod + `
lb-provider=` + lbProvider + `
keep-eip=` + strconv.FormatBool(keepEip) + `
health-check=` + healthCheck + `
health-check-option=` + HealthCheckOption + `

[NetworkingOptions]
public-network-name=` + publicNetworkName + `1
public-network-name=` + publicNetworkName + `2
internal-network-name=` + internalNetworkName + `1
internal-network-name=` + internalNetworkName + `2

[MetadataOptions]
search-order=` + searchOrder + `
`
	cfg, err := LoadELBConfig(str)
	if err != nil {
		t.Fatalf("error loadbabalancer config: %s", err)
	}

	if cfg.LoadBalancerOpts.NetworkID != networkID {
		t.Fatalf("NetworkID, expected: %v, got: %v", networkID, cfg.LoadBalancerOpts.NetworkID)
	}
	if cfg.LoadBalancerOpts.SubnetID != subnetID {
		t.Fatalf("SubnetID, expected: %v, got: %v", subnetID, cfg.LoadBalancerOpts.SubnetID)
	}
	if cfg.LoadBalancerOpts.SessionAffinityMode != lbMethod {
		t.Fatalf("SessionAffinityMode, expected: %v, got: %v", lbMethod, cfg.LoadBalancerOpts.SessionAffinityMode)
	}
	if cfg.LoadBalancerOpts.LBProvider != lbProvider {
		t.Fatalf("LBProvider, expected: %v, got: %v", lbProvider, cfg.LoadBalancerOpts.LBProvider)
	}
	if cfg.LoadBalancerOpts.KeepEIP != keepEip {
		t.Fatalf("KeepEIP, expected: %v, got: %v", keepEip, cfg.LoadBalancerOpts.KeepEIP)
	}

	publicNetworkNames := cfg.NetworkingOpts.PublicNetworkName
	if publicNetworkNames[0] != publicNetworkName+"1" || publicNetworkNames[1] != publicNetworkName+"2" {
		t.Fatalf("PublicNetworkName, expected: %v, got: %v", publicNetworkName, publicNetworkNames)
	}

	internalNetworkNames := cfg.NetworkingOpts.InternalNetworkName
	if internalNetworkNames[0] != internalNetworkName+"1" || internalNetworkNames[1] != internalNetworkName+"2" {
		t.Fatalf("InternalNetworkName, expected: %v, got: %v", internalNetworkName, internalNetworkNames)
	}

	if cfg.MetadataOpts.SearchOrder != searchOrder {
		t.Fatalf("SearchOrder, expected: %v, got: %v", searchOrder, cfg.MetadataOpts.SearchOrder)
	}
}

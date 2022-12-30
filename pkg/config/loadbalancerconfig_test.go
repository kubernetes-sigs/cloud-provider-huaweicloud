package config

import (
	"strconv"
	"testing"
)

func TestLoadELBConfigBasic(t *testing.T) {
	const (
		lbProvider = "vlb"
		lbMethod   = "ROUND_ROBIN"

		keepEip = true

		SessionAffinityFlag               = "on"
		SessionAffinityType               = "SOURCE_IP"
		SessionAffinityCookieName         = "session_id"
		SessionAffinityPersistenceTimeout = 15

		HealthCheckFlag       = "on"
		healthCheckDelay      = 5
		healthCheckTimeout    = 15
		healthCheckMaxRetries = 5

		publicNetworkName   = "public-network-name"
		internalNetworkName = "internal-network-name"

		searchOrder = "metadataService,configDrive"
	)

	data := map[string]string{
		"loadBalancerOption": `{
			"lb-algorithm": "` + lbMethod + `",
			"lb-provider": "` + lbProvider + `",
			"keep-eip": ` + strconv.FormatBool(keepEip) + `,

			"session-affinity-flag": "` + SessionAffinityFlag + `",
			"session-affinity-option": {
				"type": "` + SessionAffinityType + `",
				"cookie_name": "` + SessionAffinityCookieName + `",
				"persistence_timeout": ` + strconv.Itoa(SessionAffinityPersistenceTimeout) + `
			},
			"health-check-flag": "` + HealthCheckFlag + `",
			"health-check-option": {
				"delay": ` + strconv.Itoa(healthCheckDelay) + `,
				"timeout": ` + strconv.Itoa(healthCheckTimeout) + `,
				"max_retries": ` + strconv.Itoa(healthCheckMaxRetries) + `
			}
		}`,
		"networkingOption": `{
			"public-network-name": ["` + publicNetworkName + `"],
			"internal-network-name": ["` + internalNetworkName + `"]
		}`,
		"metadataOption": `{
			"search-order": "` + searchOrder + `"
		}`,
	}

	cfg := LoadELBConfig(data)

	if cfg.LoadBalancerOpts.LBProvider != lbProvider {
		t.Fatalf("LBProvider, expected: %v, got: %v", lbProvider, cfg.LoadBalancerOpts.LBProvider)
	}
	if cfg.LoadBalancerOpts.LBAlgorithm != lbMethod {
		t.Fatalf("LBAlgorithm, expected: %v, got: %v", lbMethod, cfg.LoadBalancerOpts.LBAlgorithm)
	}
	if cfg.LoadBalancerOpts.SessionAffinityFlag != SessionAffinityFlag {
		t.Fatalf("SessionAffinityFlag, expected: %v, got: %v", SessionAffinityFlag, cfg.LoadBalancerOpts.SessionAffinityFlag)
	}
	if cfg.LoadBalancerOpts.SessionAffinityOption.Type.Value() != SessionAffinityType {
		t.Fatalf("SessionAffinityType, expected: %v, got: %v",
			SessionAffinityType, cfg.LoadBalancerOpts.SessionAffinityOption.Type.Value())
	}
	if *cfg.LoadBalancerOpts.SessionAffinityOption.CookieName != SessionAffinityCookieName {
		t.Fatalf("SessionAffinityCookieName, expected: %v, got: %v", SessionAffinityCookieName,
			cfg.LoadBalancerOpts.SessionAffinityOption.CookieName)
	}
	if *cfg.LoadBalancerOpts.SessionAffinityOption.PersistenceTimeout != SessionAffinityPersistenceTimeout {
		t.Fatalf("SessionAffinityPersistenceTimeout, expected: %v, got: %v", SessionAffinityPersistenceTimeout, cfg.LoadBalancerOpts.SessionAffinityOption.PersistenceTimeout)
	}
	if cfg.LoadBalancerOpts.HealthCheckFlag != HealthCheckFlag {
		t.Fatalf("HealthCheckFlag, expected: %v, got: %v", HealthCheckFlag, cfg.LoadBalancerOpts.HealthCheckFlag)
	}

	publicNetworkNames := cfg.NetworkingOpts.PublicNetworkName
	if publicNetworkNames[0] != publicNetworkName {
		t.Fatalf("PublicNetworkName, expected: %v, got: %v", publicNetworkName, publicNetworkNames)
	}

	internalNetworkNames := cfg.NetworkingOpts.InternalNetworkName
	if internalNetworkNames[0] != internalNetworkName {
		t.Fatalf("InternalNetworkName, expected: %v, got: %v", internalNetworkName, internalNetworkNames)
	}

	if cfg.MetadataOpts.SearchOrder != searchOrder {
		t.Fatalf("SearchOrder, expected: %v, got: %v", searchOrder, cfg.MetadataOpts.SearchOrder)
	}
}

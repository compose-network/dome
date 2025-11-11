package configs

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestConfigPrivateKeysAreValidForECDSA verifies that the normalized private keys
// from the config can be successfully converted to ECDSA private keys.
// This is an integration test that ensures the config normalization works correctly
// with the crypto.HexToECDSA function used throughout the codebase.
func TestConfigPrivateKeysAreValidForECDSA(t *testing.T) {
	// This test uses the global Values variable which is initialized in init()
	// The init() function should have already normalized the private keys

	for chainName, cfg := range Values.L2.ChainConfigs {
		t.Run(string(chainName), func(t *testing.T) {
			// Attempt to create ECDSA private key from the normalized PK
			privateKey, err := crypto.HexToECDSA(cfg.PK)
			if err != nil {
				t.Fatalf("failed to convert private key to ECDSA for chain %s: %v", chainName, err)
			}

			// Verify we can derive an address from it
			address := crypto.PubkeyToAddress(privateKey.PublicKey)
			if address.Hex() == "0x0000000000000000000000000000000000000000" {
				t.Errorf("derived zero address for chain %s, which likely indicates an invalid private key", chainName)
			}

			t.Logf("Successfully created ECDSA key for chain %s, address: %s", chainName, address.Hex())
		})
	}
}

// TestConfigPrivateKeysDoNotHavePrefix verifies that after normalization,
// no private keys in the config have the '0x' prefix.
func TestConfigPrivateKeysDoNotHavePrefix(t *testing.T) {
	for chainName, cfg := range Values.L2.ChainConfigs {
		t.Run(string(chainName), func(t *testing.T) {
			if len(cfg.PK) >= 2 && (cfg.PK[:2] == "0x" || cfg.PK[:2] == "0X") {
				t.Errorf("private key for chain %s still has '0x' prefix after normalization: %s",
					chainName, cfg.PK[:10]+"...")
			}
		})
	}
}
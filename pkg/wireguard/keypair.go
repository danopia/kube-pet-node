package wireguard

import "os"

func (wg *WgInterface) EnsureKeyMaterial() (string, error) {
	config, err := wg.ReadPersistentConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}

		// Invent a blank config if necesary
		config = &WgQuickConfig{
			ListenPort:   -1,
			FirewallMark: -1,
		}
	}

	// If there's a private key, then just return the publickey part
	if config.PrivateKey != "" {
		privkey, err := readPrivateKeyFromBase64(config.PrivateKey)
		if err != nil {
			return "", err
		}

		pubkeyHex := privkey.publicKey().ToBase64()
		return pubkeyHex, nil
	}

	// Create new key material
	newKey, err := newPrivateKey()
	if err != nil {
		return "", err
	}

	config.PrivateKey = newKey.ToBase64()
	if err := wg.WritePersistentConfig(config); err != nil {
		return "", err
	}

	return newKey.publicKey().ToBase64(), nil
}

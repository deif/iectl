package ssh

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Option func(*ssh.ClientConfig)

func ClientConfig(opts ...Option) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// if the above options did not result in a HostKeyCallback, use our default
	if config.HostKeyCallback == nil {
		d, err := DefaultKnownHostCallback()
		if err != nil {
			return nil, fmt.Errorf("unable to setup default known hosts: %w", err)
		}
		d(config)
	}

	// the same with auth methods, we gotta have auth methods...
	if len(config.Auth) == 0 {
		d, err := DefaultSignerAuth()
		if err != nil {
			return nil, fmt.Errorf("unable to initialize default signer auth: %w", err)
		}
		d(config)
	}

	// set default user if not set
	if config.User == "" {
		user, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("unable to determine current user: %w", err)
		}
		config.User = user.Username
	}

	return config, nil
}

func WithIdentityFile(path string) (Option, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read identity file %s: %w", path, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse identity file %s: %w", path, err)
	}

	return func(cc *ssh.ClientConfig) {
		cc.Auth = append(cc.Auth, ssh.PublicKeys(signer))
	}, nil
}

func DefaultSignerAuth() (Option, error) {
	signers := make([]ssh.Signer, 0)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to determine user home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	keyFiles := []string{
		"id_rsa",
		"id_ecdsa",
		"id_ed25519",
	}

	for _, keyFile := range keyFiles {
		keyPath := filepath.Join(sshDir, keyFile)
		key, err := os.ReadFile(keyPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("unable to read private key %s: %w", keyPath, err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key %s: %w", keyPath, err)
		}

		signers = append(signers, signer)
	}

	return func(cc *ssh.ClientConfig) {
		cc.Auth = append(cc.Auth, ssh.PublicKeys(signers...))
	}, nil
}

func WithPassword(p string) Option {
	return func(cc *ssh.ClientConfig) {
		cc.Auth = append(cc.Auth, ssh.Password(p))
	}
}

func WithUser(u string) Option {
	return func(cc *ssh.ClientConfig) {
		cc.User = u
	}
}

func WithInsecureIgnoreHostkey(cc *ssh.ClientConfig) {
	cc.HostKeyCallback = ssh.InsecureIgnoreHostKey()
}

func DefaultKnownHostCallback() (Option, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to determaine home directory: %w", err)
	}

	cb, err := knownhosts.New(path.Join(dirname, ".ssh/known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("unable to parse known_hosts: %w", err)
	}

	return func(cc *ssh.ClientConfig) {
		cc.HostKeyCallback = cb
	}, nil
}

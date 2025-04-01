package lib_go_ftp

import (
	"errors"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
	"net/url"
	"time"
)

type SftpClient struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	ProxyUrl string
}

func NewSftpClient(config *Config) (*SftpClient, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	// Parse the proxy URL.
	parsedURL, err := url.Parse(config.ProxyUrl)
	if err != nil {
		return nil, err
	}

	// Extract user info if provided.
	var auth *proxy.Auth = nil
	if parsedURL.User != nil {
		username := parsedURL.User.Username()
		password, _ := parsedURL.User.Password()
		auth = &proxy.Auth{
			User:     username,
			Password: password,
		}
	}

	// Get host and port for the proxy.
	proxyAddr := parsedURL.Host

	// Create a SOCKS5 dialer using the provided proxy details.
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create SOCKS5 dialer: %v", err))
	}

	// Use the public Rebex SFTP server for testing.
	sshHost := fmt.Sprintf("%s:%d", config.Host, config.Port)
	sshUser := config.User
	sshPassword := config.Password

	// Configure the SSH client.
	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshPassword),
		},
		// NOTE: For production, replace InsecureIgnoreHostKey() with proper host key validation.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Connect to the SSH server via the proxy.
	conn, err := dialer.Dial("tcp", sshHost)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to dial SSH server through proxy: %v", err))
	}

	// Upgrade the connection to an SSH client connection.
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, sshHost, sshConfig)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to establish SSH connection: %v", err))
	}

	sshClient := ssh.NewClient(sshConn, chans, reqs)
	defer sshClient.Close()

	// Create an SFTP client over the SSH connection.
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create SFTP client: %v", err))
	}

	return &SftpClient{sshClient: sshClient, sftpClient: sftpClient}, nil
}

func (s *SftpClient) Close() error {
	if s.sshClient != nil {
		err := s.sshClient.Close()
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to close SSH client: %v", err))
		}
	}
	if s.sftpClient != nil {
		err := s.sftpClient.Close()
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to close SFTP client: %v", err))
		}
	}

	return nil
}

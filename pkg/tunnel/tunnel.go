package tunnel

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Config holds SSH tunnel configuration.
type Config struct {
	Host       string
	Port       int
	User       string
	Password   string
	KeyPath    string // 私钥文件路径
	KeyData    string // 私钥内容（与 KeyPath 二选一）
	Passphrase string
}

// OptionsFromMap extracts SSH config from a with-options map.
// Returns nil if no ssh_host is configured.
func OptionsFromMap(opts map[string]string) *Config {
	host, ok := opts["ssh_host"]
	if !ok || host == "" {
		return nil
	}
	cfg := &Config{Host: host}
	if p, err := strconv.Atoi(opts["ssh_port"]); err == nil {
		cfg.Port = p
	} else {
		cfg.Port = 22
	}
	cfg.User = opts["ssh_user"]
	if cfg.User == "" {
		cfg.User = "root"
	}
	cfg.Password = opts["ssh_password"]
	cfg.KeyPath = opts["ssh_key"]
	cfg.KeyData = opts["ssh_key_data"]
	cfg.Passphrase = opts["ssh_key_passphrase"]
	return cfg
}

// Dial establishes an SSH connection to the jump host and creates a local
// port forward to (targetHost, targetPort). Returns the local address
// (127.0.0.1:port) to connect to, and a close function.
func Dial(cfg *Config, targetHost string, targetPort int) (localAddr string, closeFn func(), err error) {
	auth, err := authMethods(cfg)
	if err != nil {
		return "", nil, fmt.Errorf("ssh auth: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return "", nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Close()
		return "", nil, fmt.Errorf("local listen: %w", err)
	}

	go func() {
		for {
			localConn, err := local.Accept()
			if err != nil {
				return
			}
			go func() {
				remoteConn, err := client.Dial("tcp", net.JoinHostPort(targetHost, strconv.Itoa(targetPort)))
				if err != nil {
					localConn.Close()
					return
				}
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					io.Copy(localConn, remoteConn)
					localConn.Close()
					wg.Done()
				}()
				go func() {
					io.Copy(remoteConn, localConn)
					remoteConn.Close()
					wg.Done()
				}()
				wg.Wait()
			}()
		}
	}()

	localAddr = fmt.Sprintf("127.0.0.1:%d", local.Addr().(*net.TCPAddr).Port)
	closeFn = func() {
		local.Close()
		client.Close()
	}
	return localAddr, closeFn, nil
}

func authMethods(cfg *Config) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if cfg.KeyData != "" || cfg.KeyPath != "" {
		var (
			keyData []byte
			err     error
		)
		if cfg.KeyData != "" {
			keyData = []byte(cfg.KeyData)
		} else {
			keyData, err = os.ReadFile(cfg.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("read ssh key %s: %w", cfg.KeyPath, err)
			}
		}
		var signer ssh.Signer
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	return methods, nil
}

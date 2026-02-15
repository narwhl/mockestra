package kanidm

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag                 = "kanidm"
	Image               = "kanidm/server"
	Port                = "8443/tcp"
	LDAPPort            = "3636/tcp"
	ContainerPrettyName = "Kanidm"

	DefaultDomain = "idm.example.com"
	DefaultOrigin = "https://idm.example.com:8443"
)

// WithDomain sets the DNS domain name of the server.
// This is used in security-critical contexts such as webauthn.
func WithDomain(domain string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_DOMAIN"] = domain
		return nil
	}
}

// WithOrigin sets the origin URL for webauthn.
// This must match or be a descendant of the domain name.
func WithOrigin(origin string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_ORIGIN"] = origin
		return nil
	}
}

// WithBindAddress sets the webserver bind address.
func WithBindAddress(addr string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_BINDADDRESS"] = addr
		return nil
	}
}

// WithLDAPBindAddress sets the LDAP server bind address.
// If set, the LDAP port will be exposed.
func WithLDAPBindAddress(addr string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_LDAPBINDADDRESS"] = addr
		// Ensure LDAP port is exposed
		hasLDAPPort := false
		for _, port := range req.ExposedPorts {
			if port == LDAPPort {
				hasLDAPPort = true
				break
			}
		}
		if !hasLDAPPort {
			req.ExposedPorts = append(req.ExposedPorts, LDAPPort)
		}
		return nil
	}
}

// WithLogLevel sets the log level of the server.
// Valid values are: info, debug, trace.
func WithLogLevel(level string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_LOG_LEVEL"] = level
		return nil
	}
}

// WithDBPath sets the path to the kanidm database.
func WithDBPath(path string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_DB_PATH"] = path
		return nil
	}
}

// WithDBFSType sets the filesystem type for database tuning.
// Valid values are: zfs, other.
func WithDBFSType(fsType string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_DB_FS_TYPE"] = fsType
		return nil
	}
}

// WithDBArcSize sets the number of entries to store in the in-memory cache.
// Minimum value is 256.
func WithDBArcSize(size int) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["KANIDM_DB_ARC_SIZE"] = fmt.Sprintf("%d", size)
		return nil
	}
}

// generateSelfSignedCert generates a self-signed TLS certificate for testing.
func generateSelfSignedCert(domain string) (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Mockestra Test"},
			CommonName:   domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain, "localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certBuf := new(bytes.Buffer)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	keyBuf := new(bytes.Buffer)
	if err := pem.Encode(keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key: %w", err)
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"kanidm_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"kanidm"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	// Generate self-signed TLS certificates for the container
	certPEM, keyPEM, err := generateSelfSignedCert(DefaultDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TLS certificates: %w", err)
	}

	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{
				Port,
			},
			Env: map[string]string{
				"KANIDM_DOMAIN":      DefaultDomain,
				"KANIDM_ORIGIN":      DefaultOrigin,
				"KANIDM_TLS_CHAIN":   "/data/chain.pem",
				"KANIDM_TLS_KEY":     "/data/key.pem",
				"KANIDM_BINDADDRESS": "0.0.0.0:8443",
				"KANIDM_DB_PATH":     "/data/kanidm.db",
			},
			Files: []testcontainers.ContainerFile{
				{
					ContainerFilePath: "/data/chain.pem",
					Reader:            bytes.NewReader(certPEM),
					FileMode:          0o644,
				},
				{
					ContainerFilePath: "/data/key.pem",
					Reader:            bytes.NewReader(keyPEM),
					FileMode:          0o600,
				},
			},
			WaitingFor: wait.ForLog("ready to rock").WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&r); err != nil {
			return nil, err
		}
	}

	return &r, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"kanidm"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"kanidm"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port: "https",
			}
			// Check if LDAP port is exposed
			for _, port := range p.Request.ExposedPorts {
				if port == LDAPPort {
					portLabels[LDAPPort] = "ldaps"
					break
				}
			}
			var ports []any
			for port, label := range portLabels {
				p, err := c.MappedPort(ctx, nat.Port(port))
				if err != nil {
					return fmt.Errorf("failed to get mapped port for %s: %w", port, err)
				}
				ports = append(ports, label)
				ports = append(ports, fmt.Sprintf("localhost:%s", p.Port()))
			}
			slog.Info(fmt.Sprintf("%s container is running", ContainerPrettyName), ports...)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			err := c.Terminate(ctx)
			if err != nil {
				slog.Warn(fmt.Sprintf("an error occurred while terminating %s container", ContainerPrettyName), "error", err)
			} else {
				slog.Info(fmt.Sprintf("%s container is terminated", ContainerPrettyName))
			}
			return err
		},
	})

	return Result{
		Container:      c,
		ContainerGroup: c,
	}, nil
}

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"kanidm"`),
		),
		Actualize,
	),
)

package web

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/gravitational/teleport"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

const (
	// HTTPS is https prefix
	HTTPS = "https"
	// WSS is secure web sockets prefix
	WSS = "wss"
)

// SSHAgentLogin issues call to web proxy and receives temp certificate
// if credentials are valid
//
// proxyAddr must be specified as host:port
func SSHAgentLogin(proxyAddr, user, password, hotpToken string, pubKey []byte, ttl time.Duration, insecure bool, pool *x509.CertPool) (*SSHLoginResponse, error) {
	// validate proxyAddr:
	host, port, err := net.SplitHostPort(proxyAddr)
	if err != nil || host == "" || port == "" {
		if err != nil {
			log.Error(err)
		}
		return nil, trace.Wrap(
			teleport.BadParameter("proxyAddress",
				fmt.Sprintf("'%v' is not a valid proxy address", proxyAddr)))
	}
	proxyAddr = "https://" + net.JoinHostPort(host, port)

	var opts []roundtrip.ClientParam

	if pool != nil {
		// use custom set of trusted CAs
		opts = append(opts, roundtrip.HTTPClient(newClientWithPool(pool)))
	} else if insecure {
		// skip https cert verification, oh no!
		fmt.Printf("WARNING: You are using insecure connection to SSH proxy %v\n", proxyAddr)
		opts = append(opts, roundtrip.HTTPClient(newInsecureClient()))
	}

	clt, err := newWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(clt.Endpoint("webapi", "ssh", "certs"), createSSHCertReq{
		User:      user,
		Password:  password,
		HOTPToken: hotpToken,
		PubKey:    pubKey,
		TTL:       ttl,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

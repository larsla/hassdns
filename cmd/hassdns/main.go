package main

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ed25519"

	"github.com/larsla/hassdns/internal/api"
	"github.com/larsla/hassdns/internal/keys"
	"github.com/larsla/hassdns/internal/validation"
)

var (
	name     = flag.String("name", "", "the name you want to claim")
	keyFile  = flag.String("key", "hassdns.key", "filename of your private key")
	url      = flag.String("url", "https://us-central1-hass-wtf.cloudfunctions.net/update", "URL of DNS service")
	daemon   = flag.Bool("daemon", false, "run as a daemon that updates according to -interval")
	interval = flag.Duration("interval", time.Minute*30, "number of minutes between updates")
)

func main() {
	flag.Parse()

	// Validate name
	if len(*name) < 4 || !validation.ValidateName(*name) {
		log.Fatal("Name needs to be minimum 4 characters long and can only contain a-z and 0-9")
	}

	// If we don't already have a private key file, create one
	_, err := os.Lstat(*keyFile)
	if err != nil {
		key, err := keys.Generate()
		if err != nil {
			log.Fatal(err)
		}

		keyStr := keys.KeyToString(key)

		if err := ioutil.WriteFile(*keyFile, []byte(keyStr), 0600); err != nil {
			log.Fatal(err)
		}
	}

	// Read private key from file
	keyBytes, err := ioutil.ReadFile(*keyFile)
	if err != nil {
		log.Fatal(err)
	}
	key, err := keys.StringToKey(string(keyBytes))
	if err != nil {
		log.Fatal(err)
	}

	// Force IPv4 by overriding the DialContext
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: false,
	}
	http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		addrParts := strings.Split(addr, ":")
		ip, err := net.ResolveIPAddr("ip4", addrParts[0])
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, fmt.Sprintf("%s:%s", ip.IP.String(), addrParts[1]))
	}

	for {

		payload := createPayload(key, *name)

		// Marshal to JSON
		body, err := json.Marshal(payload)
		if err != nil {
			log.Fatal(err)
		}

		res, err := http.Post(*url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("ERROR: %s", err)
		} else {
			resBody, _ := ioutil.ReadAll(res.Body)
			log.Printf("Response: (%s) '%s'", res.Status, string(resBody))
		}

		if !*daemon {
			break
		}

		time.Sleep(*interval)
	}
}

func createPayload(key ed25519.PrivateKey, name string) api.Update {
	// Create signature
	ts := time.Now().UTC().Unix()
	msg := fmt.Sprintf("%s,%d", name, ts)
	sig := keys.Sign(key, []byte(msg))

	// Get public key string
	publickeyStr := base32.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))

	// Build payload
	return api.Update{
		Timestamp: ts,
		PublicKey: publickeyStr,
		Subdomain: name,
		Signature: base32.StdEncoding.EncodeToString(sig),
	}
}

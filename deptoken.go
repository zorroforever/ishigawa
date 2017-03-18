package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/textproto"
	"os"
	"path"
	"time"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"

	"github.com/boltdb/bolt"
	"github.com/fullsailor/pkcs7"
	"github.com/micromdm/dep"
)

const (
	depTokenRSAKeyFilename = "deptoken.key"
	depTokenCertFilename   = "deptoken.pem"
	depTokenBucket         = "mdm.DEPToken"
)

type DEPTokenJSON struct {
	ConsumerKey       string    `json:"consumer_key"`
	ConsumerSecret    string    `json:"consumer_secret"`
	AccessToken       string    `json:"access_token"`
	AccessSecret      string    `json:"access_secret"`
	AccessTokenExpiry time.Time `json:"access_token_expiry"`
}

func depToken(args []string) error {
	flagset := flag.NewFlagSet("deptoken", flag.ExitOnError)
	var (
		flPublicKey = flagset.String("public-key", "", "filename of public key to write (to be uploaded to deploy.apple.com)")
		flTokenFile = flagset.String("token", "", "filename of p7 encrypted token file")
	)
	flagset.Usage = usageFor(flagset, "micromdm deptoken [flags]")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	keyPath := path.Join(configDBPath, depTokenRSAKeyFilename)
	var pk *rsa.PrivateKey
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// key doesn't yet exist, make it
		pk, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}
		pemBlock := pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)}

		certOut, err := os.Create(keyPath)
		if err != nil {
			return err
		}

		pem.Encode(certOut, &pemBlock)
		certOut.Close()

		// fmt.Println("generated and saved key", keyPath)
	} else {
		// key exists, load it
		pemKey, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return err
		}
		block, _ := pem.Decode(pemKey)

		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return errors.New("invalid DEP token private key")
		}

		if pk, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			return err
		}

		// fmt.Println("loaded key", keyPath)
	}

	certPath := path.Join(configDBPath, depTokenCertFilename)
	var cert []byte
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		// cert doesn't yet exist, make it
		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				CommonName: "micromdm-dep-token",
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
		}

		cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &pk.PublicKey, pk)
		if err != nil {
			return err
		}

		certOut, err := os.Create(certPath)
		pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
		certOut.Close()

		// fmt.Println("generated and saved cert", certPath)
	} else {
		// cert exists, load it
		pemCert, err := ioutil.ReadFile(certPath)
		if err != nil {
			return err
		}
		block, _ := pem.Decode(pemCert)

		if block == nil || block.Type != "CERTIFICATE" {
			return errors.New("invalid DEP token cert")
		}

		cert = block.Bytes

		if _, err = x509.ParseCertificate(cert); err != nil {
			return err
		}

		// fmt.Println("loaded cert", keyPath)
	}

	if *flPublicKey == "" && *flTokenFile == "" {
		flagset.Usage()
		return nil
	}

	if *flPublicKey != "" {
		if _, err := os.Stat(certPath); err == nil {
			return errors.New("public key filename already exists, please choose another")
		}
		certOut, err := os.Create(*flPublicKey)
		if err != nil {
			return err
		}
		pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
		certOut.Close()
		fmt.Println("wrote", *flPublicKey)
	}

	if *flTokenFile != "" {
		f, err := os.Open(*flTokenFile)
		if err != nil {
			return err
		}
		defer f.Close()

		tr := textproto.NewReader(bufio.NewReader(f))
		if _, err := tr.ReadMIMEHeader(); err != nil {
			return err
		}
		dec := base64.NewDecoder(base64.StdEncoding, tr.DotReader())
		buf := new(bytes.Buffer)
		io.Copy(buf, dec)
		p7, err := pkcs7.Parse(buf.Bytes())
		if err != nil {
			return err
		}

		parsedCert, err := x509.ParseCertificate(cert)
		if err != nil {
			return err
		}

		decrypted, err := p7.Decrypt(parsedCert, pk)
		if err != nil {
			return err
		}

		// the contained decrypted data is also wrapped in a textproto-like
		// wrapper. strip it, too.

		tr = textproto.NewReader(bufio.NewReader(bytes.NewReader(decrypted)))
		if _, err := tr.ReadMIMEHeader(); err != nil {
			return err
		}
		body, err := ioutil.ReadAll(tr.DotReader())

		// the body part of the textproto is an almost PEM-like structure.
		// unpack it as well.

		startFound := bytes.Index(body, []byte("-----BEGIN MESSAGE-----"))
		endFound := bytes.Index(body, []byte("-----END MESSAGE-----"))

		if endFound <= startFound {
			return errors.New("invalid format in decrypted message")
		}

		// finally the JSON for the token values!

		tokenJSON := body[startFound+24 : endFound-1]

		depToken := &DEPTokenJSON{}

		err = json.Unmarshal(tokenJSON, depToken)
		if err != nil {
			return err
		}

		// copy over values
		depConfig := &dep.Config{}
		depConfig.ConsumerKey = depToken.ConsumerKey
		depConfig.ConsumerSecret = depToken.ConsumerSecret
		depConfig.AccessToken = depToken.AccessToken
		depConfig.AccessSecret = depToken.AccessSecret

		sm := &config{}
		sm.setupBolt()

		err = sm.db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(depTokenBucket))
			if err == nil {
				err = b.Put([]byte(depConfig.ConsumerKey), tokenJSON)
			}
			return err
		})

		if err != nil {
			return err
		}

		fmt.Println("saved token", depConfig.ConsumerKey)
	}

	return nil
}

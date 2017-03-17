package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/pkcs12"

	"github.com/RobotsAndPencils/buford/push"
	"github.com/boltdb/bolt"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"

	boltdepot "github.com/micromdm/scep/depot/bolt"
	scep "github.com/micromdm/scep/server"

	"github.com/micromdm/nano/checkin"
	"github.com/micromdm/nano/command"
	"github.com/micromdm/nano/connect"
	"github.com/micromdm/nano/device"
	"github.com/micromdm/nano/enroll"
	"github.com/micromdm/nano/pubsub"
	nanopush "github.com/micromdm/nano/push"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	var (
		flServerURL    = flag.String("server-url", "", "public HTTPS url of your server")
		flAPNSCertPath = flag.String("apns-certificate", "mdm.p12", "path to APNS certificate")
		flAPNSKeyPass  = flag.String("apns-password", "secret", "password for your APNS cert file.")
		flAPNSKeyPath  = flag.String("apns-key", "", "path to key file if using .pem push cert")
		flTLS          = flag.Bool("tls", true, "use https")
		flTLSCert      = flag.String("tls-cert", "", "path to TLS certificate")
		flTLSKey       = flag.String("tls-key", "", "path to TLS private key")
	)
	flag.Parse()

	logger := log.NewLogfmtLogger(os.Stderr)
	stdlog.SetOutput(log.NewStdlibAdapter(logger)) // force structured logs
	mainLogger := log.NewContext(logger).With("component", "main")
	mainLogger.Log("msg", "started")

	sm := &config{
		ServerPublicURL:     *flServerURL,
		APNSCertificatePath: *flAPNSCertPath,
		APNSPrivateKeyPass:  *flAPNSKeyPass,
		APNSPrivateKeyPath:  *flAPNSKeyPath,
	}
	sm.setupPubSub()
	sm.setupBolt()
	sm.loadPushCerts()
	sm.setupSCEP(logger)
	sm.setupEnrollmentService()
	sm.setupCheckinService()
	sm.setupPushService()
	sm.setupCommandService()
	sm.setupCommandQueue()
	if sm.err != nil {
		stdlog.Fatal(sm.err)
	}

	_, err := device.NewDB(sm.db, sm.pubclient)
	if err != nil {
		stdlog.Fatal(sm.err)
	}

	ctx := context.Background()
	httpLogger := log.NewContext(logger).With("transport", "http")
	var checkinEndpoint endpoint.Endpoint
	{
		checkinEndpoint = checkin.MakeCheckinEndpoint(sm.checkinService)
	}

	checkinEndpoints := checkin.Endpoints{
		CheckinEndpoint: checkinEndpoint,
	}

	checkinOpts := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(httpLogger),
		httptransport.ServerErrorEncoder(checkin.EncodeError),
	}
	checkinHandlers := checkin.MakeHTTPHandlers(ctx, checkinEndpoints, checkinOpts...)

	pushEndpoints := nanopush.Endpoints{
		PushEndpoint: nanopush.MakePushEndpoint(sm.pushService),
	}

	commandEndpoints := command.Endpoints{
		NewCommandEndpoint: command.MakeNewCommandEndpoint(sm.commandService),
	}

	commandHandlers := command.MakeHTTPHandlers(ctx, commandEndpoints, checkinOpts...)

	pushHandlers := nanopush.MakeHTTPHandlers(ctx, pushEndpoints, checkinOpts...)
	scepHandler := scep.ServiceHandler(ctx, sm.scepService, httpLogger)
	enrollHandler := enroll.ServiceHandler(ctx, sm.enrollService, httpLogger)
	r := mux.NewRouter()
	r.Handle("/mdm/checkin", checkinHandlers.CheckinHandler).Methods("PUT")
	r.Handle("/mdm/enroll", enrollHandler)
	r.Handle("/scep", scepHandler)
	r.Handle("/push/{udid}", pushHandlers.PushHandler)
	r.Handle("/v1/commands", commandHandlers.NewCommandHandler).Methods("POST")
	srv := &http.Server{
		Addr:              ":https",
		Handler:           r,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       10 * time.Minute,
		MaxHeaderBytes:    1 << 18, // 0.25 MB
		TLSConfig:         tlsConfig(),
	}

	srvURL, err := url.Parse(sm.ServerPublicURL)
	if err != nil {
		stdlog.Fatal(err)
	}

	errs := make(chan error, 2)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go func() {
		logger := log.NewContext(logger).With("transport", "HTTP")
		if !*flTLS {
			var httpAddr = "0.0.0.0:8080"
			logger.Log("addr", httpAddr)
			errs <- http.ListenAndServe(httpAddr, r)
			return
		}

		tlsFromFile := (*flTLSCert != "" && *flTLSKey != "")
		if tlsFromFile {
			logger.Log("addr", srv.Addr)
			errs <- serveTLS(srv, *flTLSCert, *flTLSKey)
			return
		} else {
			logger.Log("addr", srv.Addr)
			errs <- serveACME(srv, srvURL.Hostname())
			return
		}
	}()

	mainLogger.Log("terminated", <-errs)
}

func serveTLS(server *http.Server, certPath, keyPath string) error {
	redirectTLS()
	err := server.ListenAndServeTLS(certPath, keyPath)
	return err
}

func serveACME(server *http.Server, domain string) error {
	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache("/var/db/le-certificates"),
	}
	server.TLSConfig.GetCertificate = m.GetCertificate
	redirectTLS()
	err := server.ListenAndServeTLS("", "")
	return err
}

// redirects port 80 to port 443
func redirectTLS() {
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Connection", "close")
			url := "https://" + req.Host + req.URL.String()
			http.Redirect(w, req, url, http.StatusMovedPermanently)
		}),
	}
	go func() { stdlog.Fatal(srv.ListenAndServe()) }()
}

func tlsConfig() *tls.Config {
	cfg := &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
	return cfg
}

type config struct {
	pubclient           *pubsub.Inmem
	db                  *bolt.DB
	pushCert            pushServiceCert
	ServerPublicURL     string
	SCEPChallenge       string
	APNSPrivateKeyPath  string
	APNSCertificatePath string
	APNSPrivateKeyPass  string

	PushService    *push.Service // bufford push
	pushService    *nanopush.Push
	checkinService checkin.Service
	enrollService  enroll.Service
	scepService    scep.Service
	commandService command.Service

	err error
}

func (c *config) setupPubSub() {
	if c.err != nil {
		return
	}
	c.pubclient = pubsub.NewInmemPubsub()
}

func (c *config) setupCommandService() {
	if c.err != nil {
		return
	}
	c.commandService, c.err = command.New(c.db, c.pubclient)
}

func (c *config) setupCommandQueue() {
	if c.err != nil {
		return
	}
	_, err := connect.NewQueue(c.db, c.pubclient)
	if err != nil {
		c.err = err
	}
}

func (c *config) setupCheckinService() {
	if c.err != nil {
		return
	}
	c.checkinService, c.err = checkin.New(c.db, c.pubclient)
}

func (c *config) setupBolt() {
	if c.err != nil {
		return
	}
	c.db, c.err = bolt.Open("mdm.db", 0777, nil)
	if c.err != nil {
		return
	}
}

func (c *config) loadPushCerts() {
	if c.err != nil {
		return
	}

	if c.APNSPrivateKeyPath == "" {
		var pkcs12Data []byte
		pkcs12Data, c.err = ioutil.ReadFile(c.APNSCertificatePath)
		if c.err != nil {
			return
		}
		c.pushCert.PrivateKey, c.pushCert.Certificate, c.err =
			pkcs12.Decode(pkcs12Data, c.APNSPrivateKeyPass)
		return
	}

	var pemData []byte
	pemData, c.err = ioutil.ReadFile(c.APNSCertificatePath)
	if c.err != nil {
		return
	}

	pemBlock, _ := pem.Decode(pemData)
	if pemBlock == nil {
		c.err = errors.New("invalid PEM data for cert")
		return
	}
	c.pushCert.Certificate, c.err = x509.ParseCertificate(pemBlock.Bytes)
	if c.err != nil {
		return
	}

	pemData, c.err = ioutil.ReadFile(c.APNSPrivateKeyPath)
	if c.err != nil {
		return
	}

	pemBlock, _ = pem.Decode(pemData)
	if pemBlock == nil {
		c.err = errors.New("invalid PEM data for privkey")
		return
	}
	c.pushCert.PrivateKey, c.err =
		x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
}

type pushServiceCert struct {
	*x509.Certificate
	PrivateKey interface{}
}

func (c *config) setupPushService() {
	if c.err != nil {
		return
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{c.pushCert.Certificate.Raw},
		PrivateKey:  c.pushCert.PrivateKey,
		Leaf:        c.pushCert.Certificate,
	}
	client, err := push.NewClient(tlsCert)
	if err != nil {
		c.err = err
		return
	}
	c.PushService = &push.Service{
		Client: client,
		Host:   push.Production,
	}

	db, err := nanopush.NewDB(c.db, c.pubclient)
	if err != nil {
		c.err = err
		return
	}
	c.pushService = nanopush.New(db, c.PushService)
}

func (c *config) setupEnrollmentService() {
	if c.err != nil {
		return
	}
	pushTopic, err := topicFromCert(c.pushCert.Certificate)
	if err != nil {
		c.err = err
		return
	}
	pub, err := url.Parse(c.ServerPublicURL)
	if err != nil {
		c.err = err
		return
	}
	SCEPRemoteURL := "https://" + strings.Split(pub.Host, ":")[0] + "/scep"

	var tlsCert string
	var SCEPCertificateSubject string
	// TODO: clean up order of inputs. Maybe pass *SCEPConfig as an arg?
	// but if you do, the packages are coupled, better not.
	c.enrollService, c.err = enroll.NewService(
		pushTopic,
		scepCACertName,
		SCEPRemoteURL,
		c.SCEPChallenge,
		c.ServerPublicURL,
		tlsCert,
		SCEPCertificateSubject,
	)
}

func topicFromCert(cert *x509.Certificate) (string, error) {
	var oidASN1UserID = asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 1}
	for _, v := range cert.Subject.Names {
		if v.Type.Equal(oidASN1UserID) {
			return v.Value.(string), nil
		}
	}

	return "", errors.New("could not find Push Topic (UserID OID) in certificate")
}

const scepCACertName = "SCEPCACert.pem"

func (c *config) setupSCEP(logger log.Logger) {
	if c.err != nil {
		return
	}

	depot, err := boltdepot.NewBoltDepot(c.db)
	if err != nil {
		c.err = err
		return
	}

	key, err := depot.CreateOrLoadKey(2048)
	if err != nil {
		c.err = err
		return
	}

	caCert, err := depot.CreateOrLoadCA(key, 5, "MicroMDM", "US")
	if err != nil {
		c.err = err
		return
	}

	c.err = savePEMCert(scepCACertName, caCert)
	if c.err != nil {
		return
	}

	opts := []scep.ServiceOption{
		scep.ClientValidity(365),
	}
	c.scepService, c.err = scep.NewService(depot, opts...)
	if c.err == nil {
		c.scepService = scep.NewLoggingService(logger, c.scepService)
	}
}

func savePEMKey(path string, key *rsa.PrivateKey) error {
	keyOutput, err := os.OpenFile(path,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600,
	)
	if err != nil {
		return err
	}
	defer keyOutput.Close()

	return pem.Encode(
		keyOutput,
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
}

func savePEMCert(path string, cert *x509.Certificate) error {
	certOutput, err := os.Create(path)
	if err != nil {
		return err
	}
	defer certOutput.Close()

	return pem.Encode(
		certOutput,
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
}

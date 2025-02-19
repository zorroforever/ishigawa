package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/micromdm/micromdm/mdm"
	"github.com/micromdm/micromdm/mdm/enroll"
	"github.com/micromdm/micromdm/pkg/crypto"
	httputil2 "github.com/micromdm/micromdm/pkg/httputil"
	"github.com/micromdm/micromdm/platform/apns"
	"github.com/micromdm/micromdm/platform/appstore"
	appsbuiltin "github.com/micromdm/micromdm/platform/appstore/builtin"
	"github.com/micromdm/micromdm/platform/blueprint"
	blueprintbuiltin "github.com/micromdm/micromdm/platform/blueprint/builtin"
	"github.com/micromdm/micromdm/platform/challenge"
	"github.com/micromdm/micromdm/platform/command"
	"github.com/micromdm/micromdm/platform/config"
	depapi "github.com/micromdm/micromdm/platform/dep"
	"github.com/micromdm/micromdm/platform/dep/sync"
	"github.com/micromdm/micromdm/platform/device"
	devicebuiltin "github.com/micromdm/micromdm/platform/device/builtin"
	"github.com/micromdm/micromdm/platform/profile"
	block "github.com/micromdm/micromdm/platform/remove"
	"github.com/micromdm/micromdm/platform/user"
	userbuiltin "github.com/micromdm/micromdm/platform/user/builtin"
	"github.com/micromdm/micromdm/server"

	"github.com/boltdb/bolt"
	"github.com/go-kit/kit/auth/basic"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/handlers"
	"github.com/groob/finalizer/logutil"
	"github.com/micromdm/go4/env"
	"github.com/micromdm/go4/httputil"
	"github.com/micromdm/go4/version"
	scep "github.com/micromdm/scep/v2/server"
	"github.com/pkg/errors"
	"golang.org/x/crypto/acme/autocert"
)

const homePage = `<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>MicroMDM</title>
  <style>
@font-face {
  font-family: system;
  font-weight: 900;
  src: local(".SFNSText-Heavy"), local(".HelveticaNeueDeskInterface-Heavy"), local(".LucidaGrandeUI"), local("Ubuntu Heavy"), local("Segoe UI Heavy"), local("Roboto-Heavy"), local("DroidSans"), local("Tahoma");
}

/* Global */
body {
  position: relative;
  font-family: "system", "Helvetica Neue", "Helvetica", "Arial", sans-serif;
  margin: 0;
  text-align: center;
  font-size: 35px;
}

.enrollment {
  background: #4a9dff;
  color: #fff;
  padding: 15px;
  border-radius: 3px;
  font-size: 30px;
  text-decoration: none;
}
  </style>
 </head>
<body>

<h1>MicroMDM</h1>
<svg xmlns="http://www.w3.org/2000/svg" width="204" height="192" viewBox="0 0 51 48" focusable="false" aria-hidden="true">
  <g fill="none" fill-rule="evenodd">
      <path fill="#366BE0" d="M34 38L0 20 34 0z"/>
      <path fill="#4A9DFF" d="M17 10l34 18-34 20z"/>
      <path fill="#80CFFF" d="M17 29L0 39l17 9zM51 28L34 38l17 9z"/>
  </g>
</svg>


<p><a class=enrollment href="mdm/enroll">Enroll a device</a></p>

</body>
</html>
`

func serve(args []string) error {
	flagset := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		flConfigPath             = flagset.String("config-path", env.String("MICROMDM_CONFIG_PATH", "/var/db/micromdm"), "Path to configuration directory")
		flServerURL              = flagset.String("server-url", env.String("MICROMDM_SERVER_URL", ""), "Public HTTPS url of your server")
		flAPIKey                 = flagset.String("api-key", env.String("MICROMDM_API_KEY", ""), "API Token for mdmctl command")
		flTLS                    = flagset.Bool("tls", env.Bool("MICROMDM_TLS", true), "Use https")
		flTLSCert                = flagset.String("tls-cert", env.String("MICROMDM_TLS_CERT", ""), "Path to TLS certificate")
		flTLSKey                 = flagset.String("tls-key", env.String("MICROMDM_TLS_KEY", ""), "Path to TLS private key")
		flHTTPAddr               = flagset.String("http-addr", env.String("MICROMDM_HTTP_ADDR", ":https"), "http(s) listen address of mdm server. defaults to :8080 if tls is false")
		flHTTPDebug              = flagset.Bool("http-debug", env.Bool("MICROMDM_HTTP_DEBUG", false), "Enable debug for http(dumps full request)")
		flHTTPProxyHeaders       = flagset.Bool("http-proxy-headers", env.Bool("MICROMDM_HTTP_PROXY_HEADERS", false), "Enable parsing of proxy headers for use behind a reverse proxy")
		flRepoPath               = flagset.String("filerepo", env.String("MICROMDM_FILE_REPO", ""), "Path to http file repo")
		flDepSim                 = flagset.String("depsim", env.String("MICROMDM_DEPSIM_URL", ""), "Use depsim URL")
		flExamples               = flagset.Bool("examples", false, "Prints some example usage")
		flCommandWebhookURL      = flagset.String("command-webhook-url", env.String("MICROMDM_WEBHOOK_URL", ""), "URL to send command responses")
		flHomePage               = flagset.Bool("homepage", env.Bool("MICROMDM_HTTP_HOMEPAGE", true), "Hosts a simple built-in webpage at the / address")
		flSCEPClientValidity     = flagset.Int("scep-client-validity", env.Int("MICROMDM_SCEP_CLIENT_VALIDITY", 365), "Sets the scep certificate validity in days")
		flNoCmdHistory           = flagset.Bool("no-command-history", env.Bool("MICROMDM_NO_COMMAND_HISTORY", false), "disables saving of command history")
		flUseDynChallenge        = flagset.Bool("use-dynamic-challenge", env.Bool("MICROMDM_USE_DYNAMIC_CHALLENGE", false), "require dynamic SCEP challenges")
		flGenDynChalEnroll       = flagset.Bool("gen-dynamic-challenge", env.Bool("MICROMDM_GEN_DYNAMIC_CHALLENGE", false), "generate dynamic SCEP challenges in enrollment profile (built-in only)")
		flValidateSCEPIssuer     = flagset.Bool("validate-scep-issuer", env.Bool("MICROMDM_VALIDATE_SCEP_ISSUER", false), "validate only the issuer of the SCEP certificate rather than the whole certificate")
		flUDIDCertAuthWarnOnly   = flagset.Bool("udid-cert-auth-warn-only", env.Bool("MICROMDM_UDID_CERT_AUTH_WARN_ONLY", false), "warn only for udid cert mismatches")
		flValidateSCEPExpiration = flagset.Bool("validate-scep-expiration", env.Bool("MICROMDM_VALIDATE_SCEP_EXPIRATION", false), "validate that the SCEP certificate is still valid")
		flPrintArgs              = flagset.Bool("print-flags", false, "Print all flags and their values")
		flQueue                  = flagset.String("queue", env.String("MICROMDM_QUEUE", "builtin"), "command queue type")
		flDMURL                  = flagset.String("dm", env.String("DM", ""), "URL to send Declarative Management requests to")
		flLogTime                = flagset.Bool("log-time", false, "Include timestamp in log messages")
		flP7Skew                 = flagset.Int("device-signature-skew", env.Int("MICROMDM_DEVICE_SIGNATURE_SKEW", 0), "Sets the allowable clock skew (in seconds) when verifying device signatures")
		flDisableRedirect        = flagset.Bool("disable-redirect", env.Bool("MICROMDM_DISABLE_REDIRECT", false), "disable the :80 -> :443 redirect if listening on :443")
	)
	flagset.Usage = usageFor(flagset, "micromdm serve [flags]")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	if *flPrintArgs {
		flagset.VisitAll(func(fl *flag.Flag) {
			fmt.Printf("%v: %v\n", fl.Usage, fl.Value)
		})
	}

	if *flExamples {
		printExamples()
		return nil
	}

	if *flServerURL == "" {
		return errors.New("must supply -server-url")
	}
	if !strings.HasPrefix(*flServerURL, "https://") {
		return errors.New("-server-url must begin with https://")
	}
	if !*flTLS && (*flTLSCert != "" || *flTLSKey != "") {
		return errors.New("cannot set -tls=false and supply -tls-cert or -tls-key")
	}

	logger := log.NewLogfmtLogger(os.Stderr)
	if *flLogTime {
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	}
	stdlog.SetOutput(log.NewStdlibAdapter(logger)) // force structured logs
	mainLogger := log.With(logger, "component", "main")
	mainLogger.Log("msg", "started")

	if err := os.MkdirAll(*flConfigPath, 0755); err != nil {
		return errors.Wrapf(err, "creating config directory %s", *flConfigPath)
	}
	sm := &server.Server{
		ConfigPath:             *flConfigPath,
		ServerPublicURL:        strings.TrimRight(*flServerURL, "/"),
		Depsim:                 *flDepSim,
		TLSCertPath:            *flTLSCert,
		CommandWebhookURL:      *flCommandWebhookURL,
		NoCmdHistory:           *flNoCmdHistory,
		UseDynSCEPChallenge:    *flUseDynChallenge,
		GenDynSCEPChallenge:    *flGenDynChalEnroll,
		ValidateSCEPIssuer:     *flValidateSCEPIssuer,
		UDIDCertAuthWarnOnly:   *flUDIDCertAuthWarnOnly,
		ValidateSCEPExpiration: *flValidateSCEPExpiration,

		WebhooksHTTPClient: &http.Client{Timeout: time.Second * 30},

		SCEPClientValidity: *flSCEPClientValidity,
		Queue:              *flQueue,
		DMURL:              *flDMURL,
	}
	if !sm.UseDynSCEPChallenge {
		// TODO: we have a static SCEP challenge password here to prevent
		// being prompted for the SCEP challenge which happens in a "normal"
		// (non-DEP) enrollment. While security is not improved it is at least
		// no less secure and prevents a useless dialog from showing.
		sm.SCEPChallenge = "micromdm"
	}

	if err := sm.Setup(logger); err != nil {
		stdlog.Fatal(err)
	}
	syncer, err := sm.CreateDEPSyncer(logger)
	if err != nil {
		stdlog.Fatal(err)
	}

	var removeService block.Service
	{
		svc, err := block.New(sm.RemoveDB)
		if err != nil {
			stdlog.Fatal(err)
		}
		removeService = block.LoggingMiddleware(logger)(svc)
	}

	devDB, err := devicebuiltin.NewDB(sm.DB)
	if err != nil {
		stdlog.Fatal(err)
	}

	devWorker := device.NewWorker(devDB, sm.PubClient, logger)
	go devWorker.Run(context.Background())

	userDB, err := userbuiltin.NewDB(sm.DB)
	if err != nil {
		stdlog.Fatal(err)
	}
	userWorker := user.NewWorker(userDB, sm.PubClient, logger)
	go userWorker.Run(context.Background())

	bpDB, err := blueprintbuiltin.NewDB(sm.DB, sm.ProfileDB)
	if err != nil {
		stdlog.Fatal(err)
	}

	blueprintWorker := blueprint.NewWorker(
		bpDB,
		userDB,
		sm.ProfileDB,
		sm.CommandService,
		sm.PubClient,
		logger,
	)
	go blueprintWorker.Run(context.Background())

	ctx := context.Background()
	httpLogger := log.With(logger, "transport", "http")

	appDB := &appsbuiltin.Repo{Path: *flRepoPath}

	scepEndpoints := scep.MakeServerEndpoints(sm.SCEPService)
	scepComponentLogger := log.With(logger, "component", "scep")
	scepEndpoints.GetEndpoint = scep.EndpointLoggingMiddleware(scepComponentLogger)(scepEndpoints.GetEndpoint)
	scepEndpoints.PostEndpoint = scep.EndpointLoggingMiddleware(scepComponentLogger)(scepEndpoints.PostEndpoint)
	scepHandler := scep.MakeHTTPHandler(scepEndpoints, sm.SCEPService, scepComponentLogger)

	pkcs7Verifier := &crypto.PKCS7Verifier{MaxSkew: time.Duration(*flP7Skew) * time.Second}

	enrollHandlers := enroll.MakeHTTPHandlers(ctx, enroll.MakeServerEndpoints(sm.EnrollService, sm.SCEPDepot), pkcs7Verifier, httptransport.ServerErrorLogger(httpLogger))

	r, options := httputil2.NewRouter(logger)

	r.Handle("/version", version.Handler())
	r.Handle("/mdm/enroll", enrollHandlers.EnrollHandler).Methods("GET", "POST")
	r.Handle("/ota/enroll", enrollHandlers.OTAEnrollHandler)
	r.Handle("/ota/phase23", enrollHandlers.OTAPhase2Phase3Handler).Methods("POST")
	r.Handle("/scep", scepHandler)
	if *flHomePage {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, homePage)
		})
	}

	mdmEndpoints := mdm.MakeServerEndpoints(sm.MDMService)
	mdm.RegisterHTTPHandlers(r, mdmEndpoints, pkcs7Verifier, logger)

	// API commands. Only handled if the user provides an api key.
	if *flAPIKey != "" {
		basicAuthEndpointMiddleware := basic.AuthMiddleware("micromdm", *flAPIKey, "micromdm")

		configsvc := config.New(sm.ConfigDB)
		configEndpoints := config.MakeServerEndpoints(configsvc, basicAuthEndpointMiddleware)
		config.RegisterHTTPHandlers(r, configEndpoints, options...)

		apnsEndpoints := apns.MakeServerEndpoints(sm.APNSPushService, basicAuthEndpointMiddleware)
		apns.RegisterHTTPHandlers(r, apnsEndpoints, options...)

		devicesvc := device.New(devDB)
		deviceEndpoints := device.MakeServerEndpoints(devicesvc, basicAuthEndpointMiddleware)
		device.RegisterHTTPHandlers(r, deviceEndpoints, options...)

		profilesvc := profile.New(sm.ProfileDB)
		profileEndpoints := profile.MakeServerEndpoints(profilesvc, basicAuthEndpointMiddleware)
		profile.RegisterHTTPHandlers(r, profileEndpoints, options...)

		blueprintsvc := blueprint.New(bpDB)
		blueprintEndpoints := blueprint.MakeServerEndpoints(blueprintsvc, basicAuthEndpointMiddleware)
		blueprint.RegisterHTTPHandlers(r, blueprintEndpoints, options...)

		blockEndpoints := block.MakeServerEndpoints(removeService, basicAuthEndpointMiddleware)
		block.RegisterHTTPHandlers(r, blockEndpoints, options...)

		usersvc := user.New(userDB)
		userEndpoints := user.MakeServerEndpoints(usersvc, basicAuthEndpointMiddleware)
		user.RegisterHTTPHandlers(r, userEndpoints, options...)

		appsvc := appstore.New(appDB)
		appEndpoints := appstore.MakeServerEndpoints(appsvc, basicAuthEndpointMiddleware)
		appstore.RegisterHTTPHandlers(r, appEndpoints, options...)

		commandEndpoints := command.MakeServerEndpoints(sm.CommandService, basicAuthEndpointMiddleware)
		command.RegisterHTTPHandlers(r, commandEndpoints, options...)

		var dc depapi.DEPClient
		if sm.DEPClient != nil {
			dc = sm.DEPClient
		}
		depsvc := depapi.New(dc, sm.PubClient)
		depsvc.Run()
		depEndpoints := depapi.MakeServerEndpoints(depsvc, basicAuthEndpointMiddleware)
		depapi.RegisterHTTPHandlers(r, depEndpoints, options...)

		depsyncEndpoints := sync.MakeServerEndpoints(sync.NewService(syncer, sm.SyncDB), basicAuthEndpointMiddleware)
		sync.RegisterHTTPHandlers(r, depsyncEndpoints, options...)

		if sm.SCEPChallengeDepot != nil {
			challengeEndpoints := challenge.MakeServerEndpoints(challenge.NewService(sm.SCEPChallengeDepot), basicAuthEndpointMiddleware)
			challenge.RegisterHTTPHandlers(r, challengeEndpoints, options...)
		}

		r.HandleFunc("/boltbackup", httputil2.RequireBasicAuth(boltBackup(sm.DB), "micromdm", *flAPIKey, "micromdm"))
	} else {
		mainLogger.Log("msg", "no api key specified")
	}

	if *flRepoPath != "" {
		if _, err := os.Stat(*flRepoPath); os.IsNotExist(err) {
			stdlog.Fatal(err)
		}
		r.PathPrefix("/repo/").Handler(http.StripPrefix("/repo/", http.FileServer(http.Dir(*flRepoPath))))
	}

	var handler http.Handler
	if *flHTTPDebug {
		handler = httputil.HTTPDebugMiddleware(os.Stdout, true, logger.Log)(r)
	} else {
		handler = r
	}
	if *flHTTPProxyHeaders {
		handler = handlers.ProxyHeaders(handler)
	}
	handler = logutil.NewHTTPLogger(httpLogger).Middleware(handler)

	srvURL, err := url.Parse(sm.ServerPublicURL)
	if err != nil {
		return errors.Wrapf(err, "parsing serverURL %q", sm.ServerPublicURL)
	}

	serveOpts := serveOptions(
		handler,
		*flHTTPAddr,
		srvURL.Hostname(),
		logger,
		*flTLSCert,
		*flTLSKey,
		sm.ConfigPath,
		*flTLS,
		*flDisableRedirect,
	)
	err = httputil.ListenAndServe(serveOpts...)
	return errors.Wrap(err, "calling ListenAndServe")
}

// serveOptions configures the []httputil.Options for ListenAndServe
func serveOptions(
	handler http.Handler,
	addr string,
	hostname string,
	logger log.Logger,
	certPath string,
	keyPath string,
	configPath string,
	tls bool,
	disableRedirect bool,
) []httputil.Option {
	tlsFromFile := (certPath != "" && keyPath != "")
	serveOpts := []httputil.Option{
		httputil.WithACMEHosts([]string{hostname}),
		httputil.WithLogger(logger),
		httputil.WithHTTPHandler(handler),
	}
	if tlsFromFile {
		serveOpts = append(serveOpts, httputil.WithKeyPair(certPath, keyPath))
	}
	if !tls && addr == ":https" {
		serveOpts = append(serveOpts, httputil.WithAddress(":8080"))
	}
	if tls {
		serveOpts = append(serveOpts, httputil.WithAutocertCache(autocert.DirCache(filepath.Join(configPath, "le-certificates"))))
	}
	if addr != ":https" {
		serveOpts = append(serveOpts, httputil.WithAddress(addr))
	}
	if disableRedirect {
		serveOpts = append(serveOpts, httputil.WithDisableRedirect(true))
	}
	return serveOpts
}

func boltBackup(db *bolt.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := db.View(func(tx *bolt.Tx) error {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="micromdm.db"`)
			w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
			_, err := tx.WriteTo(w)
			return err
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func printExamples() {
	const exampleText = `
		Quickstart:
		micromdm serve -server-url=https://my-server-url -tls-cert tls.crt -tls-key tls.key
		`
	fmt.Println(exampleText)
}

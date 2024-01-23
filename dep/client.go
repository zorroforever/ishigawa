package dep

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/go-oauth/oauth"
	"github.com/pkg/errors"
)

var version = "dev"

const (
	defaultBaseURL               = "https://mdmenrollment.apple.com"
	defaultBaseURL2              = "https://deviceservices-external.apple.com"
	mediaType                    = "application/json;charset=UTF8"
	mediaType2                   = "application/x-www-form-urlencoded;charset=UTF8"
	XServerProtocolVersionHeader = "X-Server-Protocol-Version"
	XServerProtocolVersion       = "3"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Option func(*Client)

func WithServerURL(baseURL *url.URL) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithHTTPClient(client HTTPClient) Option {
	return func(c *Client) {
		c.client = client
	}
}

type Client struct {
	consumerKey      string //given by apple
	consumerSecret   string //given by apple
	accessToken      string //given by apple
	accessSecret     string //given by apple
	authSessionToken string //requested from DEP using above credentials
	sessionExpires   time.Time

	userAgent string
	client    HTTPClient

	baseURL      *url.URL
	sessionMutex sync.Mutex
}

type OAuthParameters struct {
	ConsumerKey    string `json:"consumer_key"`
	ConsumerSecret string `json:"consumer_secret"`
	AccessToken    string `json:"access_token"`
	AccessSecret   string `json:"access_secret"`
}

func NewClient(p OAuthParameters, opts ...Option) *Client {
	baseURL, _ := url.Parse(defaultBaseURL)
	client := Client{
		consumerKey:    p.ConsumerKey,
		consumerSecret: p.ConsumerSecret,
		accessToken:    p.AccessToken,
		accessSecret:   p.AccessSecret,
		client:         http.DefaultClient,
		userAgent:      path.Join("micromdm", version),
		baseURL:        baseURL,
	}
	for _, optFn := range opts {
		optFn(&client)
	}
	return &client
}

func NewClient2(p OAuthParameters, opts ...Option) *Client {
	baseURL, _ := url.Parse(defaultBaseURL2)
	client := Client{
		consumerKey:    p.ConsumerKey,
		consumerSecret: p.ConsumerSecret,
		accessToken:    p.AccessToken,
		accessSecret:   p.AccessSecret,
		client:         http.DefaultClient,
		userAgent:      path.Join("micromdm", version),
		baseURL:        baseURL,
	}
	for _, optFn := range opts {
		optFn(&client)
	}
	return &client
}

func (c *Client) session() error {
	// making sure the session creation is thread safe if the client is used amongst multiple go routine
	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()
	if c.authSessionToken == "" {
		if err := c.newSession(); err != nil {
			return errors.Wrap(err, "creating new auth session for dep")
		}
	}

	if time.Now().After(c.sessionExpires) {
		if err := c.newSession(); err != nil {
			return errors.Wrap(err, "refreshing expired dep session")
		}
	}
	return nil
}

func (c *Client) newSession() error {
	var authSessionToken struct {
		AuthSessionToken string `json:"auth_session_token"`
	}
	consumerCredentials := oauth.Credentials{
		Token:  c.consumerKey,
		Secret: c.consumerSecret,
	}

	accessCredentials := &oauth.Credentials{
		Token:  c.accessToken,
		Secret: c.accessSecret,
	}
	form := url.Values{}

	rel, err := url.Parse("/session")
	if err != nil {
		return err
	}
	sessionURL := c.baseURL.ResolveReference(rel)

	oauthClient := oauth.Client{
		SignatureMethod: oauth.HMACSHA1,
		TokenRequestURI: sessionURL.String(),
		Credentials:     consumerCredentials,
	}

	// create request
	req, err := http.NewRequest("GET", oauthClient.TokenRequestURI, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	// set Authorization Header
	if err := oauthClient.SetAuthorizationHeader(
		req.Header,
		accessCredentials,
		"GET",
		req.URL,
		form,
	); err != nil {
		return err
	}
	// add headers
	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add(XServerProtocolVersionHeader, XServerProtocolVersion)

	// get Authorization Header
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// check resp statuscode
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("establishing DEP session: %v", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&authSessionToken); err != nil {
		return errors.Wrap(err, "decode authSessionToken from response")
	}

	// set token and expiration value
	c.authSessionToken = authSessionToken.AuthSessionToken
	c.sessionExpires = time.Now().Add(time.Minute * 3)
	return nil
}

// NewRequest creates a DEP request
func (c *Client) newRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	logger := log.NewLogfmtLogger(os.Stderr)
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, errors.Wrapf(err, "parse dep request url %s", urlStr)
	}

	u := c.baseURL.ResolveReference(rel)
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, errors.Wrap(err, "encode http body for DEP request")
		}
	}
	level.Info(logger).Log(
		"msg", "newRequest url",
		"url", u.String(),
		"body", &buf,
	)
	req, err := http.NewRequest(method, u.String(), &buf)
	if err != nil {
		return nil, errors.Wrapf(err, "creating %s request to dep %s", method, u.String())
	}

	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add(XServerProtocolVersionHeader, XServerProtocolVersion)
	return req, nil
}

func readCertFile(url string) string {
	b, _ := ioutil.ReadFile(url)
	pem.Decode(b)
	var pemBlocks []*pem.Block
	var v *pem.Block
	var pkey []byte
	for {
		v, b = pem.Decode(b)
		if v == nil {
			break
		}
		if v.Type == "PRIVATE KEY" {
			pkey = pem.EncodeToMemory(v)
			return string(pkey)
		} else {
			pemBlocks = append(pemBlocks, v)
			bytes := pem.EncodeToMemory(pemBlocks[0])
			return string(bytes)
		}
	}
	return ""
}

func (c *Client) newRequest2(method, urlStr string, formEncodedData string) (*http.Request, error) {
	logger := log.NewLogfmtLogger(os.Stderr)

	var pem = "/opt/micromdm/server/mdm-certificates/MDM_ McMurtrie Consulting LLC_Certificate.pem"
	var privateKey = "/opt/micromdm/server/mdm-certificates/mdmcert.download.push.key"
	keyString := readCertFile(privateKey)
	CertString := readCertFile(pem)
	//fmt.Printf("Cert :\n %s \n Key:\n %s \n ", CertString, keyString)
	certPair, _ := tls.X509KeyPair([]byte(CertString), []byte(keyString))
	cfg := &tls.Config{
		Certificates: []tls.Certificate{certPair},
	}
	tr := &http.Transport{
		TLSClientConfig: cfg,
	}
	client := &http.Client{Transport: tr}
	c.client = client

	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, errors.Wrapf(err, "parse dep request url %s", urlStr)
	}

	u := c.baseURL.ResolveReference(rel)
	//var buf bytes.Buffer
	if err != nil {
		return nil, errors.Wrap(err, "encode http body for DEP request")
	}

	level.Info(logger).Log(
		"msg", "newRequest2 url",
		"url", u.String(),
		"body", formEncodedData,
	)

	req, err := http.NewRequest(method, u.String(), strings.NewReader(formEncodedData))
	if err != nil {
		return nil, errors.Wrapf(err, "creating %s request to dep %s", method, u.String())
	}

	//req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", mediaType2)
	//req.Header.Add("Accept", mediaType2)
	//req.Header.Add(XServerProtocolVersionHeader, XServerProtocolVersion)
	return req, nil
}

func (c *Client) do(req *http.Request, into interface{}) error {
	if err := c.session(); err != nil {
		return errors.Wrapf(err, "get session for request to %s", c.baseURL.String())
	}
	req.Header.Add("X-ADM-Auth-Session", c.authSessionToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "perform dep request")
	}
	defer resp.Body.Close()
	logger := log.NewLogfmtLogger(os.Stderr)
	level.Debug(logger).Log(
		"msg=", "====================== do1",
		"code=", resp.StatusCode,
		"X-ADM-Auth-Session=", c.authSessionToken,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("unexpected dep response. status=%d DEP API Error: %s", resp.StatusCode, string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(into)
	return errors.Wrap(err, "decode DEP response body")

}
func (c *Client) do2(req *http.Request, into interface{}) error {
	//if err := c.session(); err != nil {
	//	return errors.Wrapf(err, "get session for request to %s", c.baseURL.String())
	//}
	//req.Header.Add("X-ADM-Auth-Session", c.authSessionToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "perform dep request")
	}
	defer resp.Body.Close()
	logger := log.NewLogfmtLogger(os.Stderr)
	bbb, _ := ioutil.ReadAll(resp.Body)
	level.Debug(logger).Log(
		"msg=", "====================== do2",
		"body=", string(bbb),
		"code=", resp.StatusCode,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("unexpected dep response. status=%d DEP API Error: %s", resp.StatusCode, string(body))
	}
	var responseData = `{"response_status":"success"}`
	//err = xml.NewDecoder(resp.Body).Decode(into)
	err = json.Unmarshal([]byte(responseData), into)
	return errors.Wrap(err, "decode DEP response body")

}

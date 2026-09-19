// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/report"
)

// UploadCmd collects (or reads) a report and uploads it to a HostCheck Gateway
// over HTTP(S). The report source follows the same convention as diff: a file
// path, "-" for stdin, or (default) a fresh local-agent scan.
//
//	hostcheck upload -url https://gw.example.com -token hcgw_XXX             # fresh scan
//	hostcheck upload -url https://gw.example.com -token hcgw_XXX -format gob  # gob
//	hostcheck collect | hostcheck upload -url http://localhost:2000 -token hcgw_XXX -
//	hostcheck upload -url https://gw.example.com -token hcgw_XXX report.json
//
// Auth: the gateway authenticates uploads with an account API token, sent as a
// Bearer credential (-token). The -client-cert/-client-key flags optionally add a
// TLS client certificate for gateways configured to require mTLS as well.
type UploadCmd struct {
	url        string
	token      string
	format     string
	enable     string
	disable    string
	timeout    time.Duration
	insecure   bool
	clientCert string
	clientKey  string
}

func (c *UploadCmd) Purpose() string {
	return "Collect a report and upload it to a HostCheck Gateway"
}

func (c *UploadCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	fset := new(flag.FlagSet)
	fset.StringVar(&c.url, "url", "", "gateway base URL, e.g. https://gateway.example.com (required)")
	fset.StringVar(&c.token, "token", "", "account API token (from the gateway) sent as a Bearer credential")
	fset.StringVar(&c.format, "format", "json", "upload encoding: json or gob")
	fset.StringVar(&c.enable, "enable", "", "comma-separated module keys to force on (fresh scan only)")
	fset.StringVar(&c.disable, "disable", "", "comma-separated module keys to force off (fresh scan only)")
	fset.DurationVar(&c.timeout, "timeout", 30*time.Second, "upload timeout")
	fset.BoolVar(&c.insecure, "insecure", false, "skip TLS certificate verification (dev only; e.g. a self-signed gateway)")
	fset.StringVar(&c.clientCert, "client-cert", "", "path to a TLS client certificate for mTLS")
	fset.StringVar(&c.clientKey, "client-key", "", "path to the TLS client certificate key")
	return "upload", fset, c.run
}

func (c *UploadCmd) run(ctx context.Context, args []string) error {
	if c.url == "" {
		return fmt.Errorf("-url is required (the gateway base URL)")
	}
	format := strings.ToLower(strings.TrimSpace(c.format))
	if format != "json" && format != "gob" {
		return fmt.Errorf("-format must be %q or %q", "json", "gob")
	}
	if (c.clientCert == "") != (c.clientKey == "") {
		return fmt.Errorf("both -client-cert and -client-key are required for mTLS")
	}

	var input string
	if len(args) > 0 {
		input = args[0]
	}
	rep, err := readReport(ctx, input, configFrom(c.enable, c.disable))
	if err != nil {
		return err
	}

	body, contentType, err := encodeReport(rep, format)
	if err != nil {
		return err
	}
	endpoint, err := uploadURL(c.url)
	if err != nil {
		return err
	}
	client, err := c.httpClient()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway rejected upload (%s): %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	fmt.Fprintf(cli.Stdout(ctx), "uploaded to %s: %s\n", endpoint, strings.TrimSpace(string(respBody)))
	return nil
}

// encodeReport serializes the report in the chosen format and returns the body
// and its Content-Type. The gateway content-negotiates on this header.
func encodeReport(rep *report.Report, format string) ([]byte, string, error) {
	if format == "gob" {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(*rep); err != nil {
			return nil, "", fmt.Errorf("gob-encoding report: %w", err)
		}
		return buf.Bytes(), "application/gob", nil
	}
	data, err := json.Marshal(rep)
	if err != nil {
		return nil, "", fmt.Errorf("json-encoding report: %w", err)
	}
	return data, "application/json", nil
}

// uploadURL resolves the gateway base URL to the report-upload endpoint. It
// accepts a bare base ("https://gw.example.com") and appends the well-known path,
// or a URL already pointing at the endpoint.
func uploadURL(base string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", fmt.Errorf("invalid -url %q: %w", base, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid -url %q: expected scheme://host[:port]", base)
	}
	const path = "/api/upload"
	if strings.TrimRight(u.Path, "/") != path {
		u.Path = strings.TrimRight(u.Path, "/") + path
	}
	return u.String(), nil
}

func (c *UploadCmd) httpClient() (*http.Client, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: c.insecure}
	if c.clientCert != "" {
		cert, err := tls.LoadX509KeyPair(c.clientCert, c.clientKey)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return &http.Client{
		Timeout:   c.timeout,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

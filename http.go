/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"context"
	"net"
	"net/http"
	"time"
)

const httpAttempts = 3

var (
	httpClient     = newHttpClient("tcp")
	httpClientIPv4 = newHttpClient("tcp4")
)

func newHttpClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}
	transport.TLSHandshakeTimeout = 15 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport, Timeout: 5 * time.Minute}
}

func httpGet(url string) (*http.Response, error) {
	var err error
	for attempt := 1; attempt <= httpAttempts; attempt++ {
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)

		client := httpClient
		if attempt > 1 {
			client = httpClientIPv4
		}

		var res *http.Response
		if res, err = client.Do(req); err == nil {
			return res, nil
		}

		if attempt < httpAttempts {
			Log.Warn("Request to", url, "failed:", err, "- retrying over IPv4")
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, err
}

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
)

func initHandler() {
	var _certs []tls.Certificate
	for _, _c := range _config.Certificate {
		_cert, err := _c.load()
		if err != nil {
			log.Println("load certificate failed", err)
			continue
		}
		_certs = append(_certs, _cert)

	}
	_tlsconfig := &tls.Config{
		Certificates: _certs,
	}
	_tlsconfig.BuildNameToCertificate()

	for _, _l := range _config.Listen {
		l, err := tls.Listen("tcp", _l, _tlsconfig)
		if err != nil {
			log.Fatal(err)
		}

		go func(l net.Listener) {
			defer l.Close()
			for {
				c, err := l.Accept()
				if err != nil {
					break
				}
				go handleConnection(c)
			}
		}(l)
	}
}

func handleConnection(c net.Conn) {
	defer c.Close()

	log.Printf("connection from %s", c.RemoteAddr().String())
	tlsconn := c.(*tls.Conn)
	connstate := tlsconn.ConnectionState()
	for !connstate.HandshakeComplete {
		if err := tlsconn.Handshake(); err != nil {
			log.Println(err)
			return
		}
		connstate = tlsconn.ConnectionState()
	}

	log.Printf("handshake complete")
	servername := connstate.ServerName
	var backend *url.URL
	for _, f := range _config.Forward {
		if !f.match(servername) {
			continue
		}
		backend = f.getBackend()
		break
	}
	if backend == nil {
		_b, err := url.Parse(_config.DefaultBackend)
		if err != nil {
			log.Println(err)
			return
		}
		backend = _b
	}
	log.Printf("sni name %s, get backend: %s", servername, backend.String())
	handleForward(tlsconn, backend)
}

func handleForward(c *tls.Conn, b *url.URL) {
	var remote net.Conn
	var err error

	log.Printf("forward to %s", b.String())
	switch b.Scheme {
	case "tcp":
		remote, err = net.Dial("tcp", b.Host)
	case "unix":
		remote, err = net.Dial("unix", b.Host)
	case "http":
		if !strings.Contains(b.Host, ":") {
			b.Host = fmt.Sprintf("%s:80", b.Host)
		}
		remote, err = net.Dial("tcp", b.Host)
	case "tls":
		h, _, _ := net.SplitHostPort(b.Host)
		remote, err = tls.Dial("tcp", b.Host, &tls.Config{ServerName: h})
	}

	if err != nil {
		log.Println(err)
		return
	}
	log.Println("begin data forward")

	defer remote.Close()
	ch := make(chan struct{}, 2)
	go func() {
		io.Copy(c, remote)
		ch <- struct{}{}
	}()
	go func() {
		io.Copy(remote, c)
		ch <- struct{}{}
	}()
	<-ch
}

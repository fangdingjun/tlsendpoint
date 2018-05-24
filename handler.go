package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/fangdingjun/go-log"
)

func initHandler() {
	var _certs []tls.Certificate
	for _, _c := range _config.Certificate {
		log.Debugf("load certificate %s, %s...", _c.Cert, _c.Key)
		_cert, err := _c.load()
		if err != nil {
			log.Errorln("load certificate failed", err)
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
			log.Fatalf("listen on %s error: %s", _l, err)
		}
		log.Printf("Listen on %s", l.Addr().String())
		go func(l net.Listener) {
			defer l.Close()
			for {
				c, err := l.Accept()
				if err != nil {
					log.Errorf("accept error: %s", err)
					continue
				}
				go handleConnection(c)
			}
		}(l)
	}
}

func handleConnection(c net.Conn) {
	defer func() {
		log.Debugf("close connection %s", c.RemoteAddr())
		if err := c.Close(); err != nil {
			log.Debugf("close connection %s error: %s", c.RemoteAddr(), err)
		}
	}()

	log.Debugf("connection from %s\n", c.RemoteAddr().String())
	tlsconn := c.(*tls.Conn)

	connstate := tlsconn.ConnectionState()
	for !connstate.HandshakeComplete {
		if err := tlsconn.Handshake(); err != nil {
			log.Errorf("%s tls handshake error: %s", c.RemoteAddr(), err)
			return
		}
		connstate = tlsconn.ConnectionState()
	}

	log.Debugf("%s handshake complete", c.RemoteAddr())

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
			log.Errorf("parse addr error: %s", err)
			return
		}
		backend = _b
	}

	log.Debugf("connection from %s, tls version 0x%x, sni: %s, forward to: %s",
		c.RemoteAddr().String(),
		connstate.Version,
		servername,
		backend.String(),
	)

	handleForward(tlsconn, backend)
}

func handleForward(c *tls.Conn, b *url.URL) {
	var remote net.Conn
	var err error

	log.Debugf("%s forward to %s", c.RemoteAddr(), b.String())
	switch b.Scheme {
	case "http":
		httpForward(c, b)
		return
	case "tcp":
		//log.Debugf("forward to tcp backend %s", b.Host)
		remote, err = net.Dial("tcp", b.Host)
	case "unix":
		//log.Debugf("forward to unix backend %s", b.Host)
		remote, err = net.Dial("unix", b.Host)
	case "tls":
		//log.Debugf("forward to tls backend %s", b.Host)
		h, _, _ := net.SplitHostPort(b.Host)
		remote, err = tls.Dial("tcp", b.Host, &tls.Config{ServerName: h})
	default:
		log.Errorf("backend type '%s' is not supported", b.Scheme)
		return
	}

	if err != nil {
		log.Errorf("dail to backend %s error: %s", b.String(), err)
		return
	}

	if remote == nil {
		log.Warnln("remote is nil")
		return
	}
	//log.Println("begin data forward")

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

func writeErrResponse(w io.Writer, status int, content string) {
	hdr := http.Header{}
	hdr.Add("Connection", "close")
	hdr.Add("Server", "nginx/1.10.1")
	hdr.Add("Content-Type", "text/plain")

	r := bytes.NewBuffer([]byte(content))
	body := ioutil.NopCloser(r)

	res := &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        hdr,
		Body:          body,
		ContentLength: int64(len(content)),
	}
	res.Write(w)
}

func httpForward(r net.Conn, b *url.URL) {
	//log.Debugf("forward to http backend %s", b.Host)
	if !strings.Contains(b.Host, ":") {
		b.Host = fmt.Sprintf("%s:80", b.Host)
	}
	backend, err := net.Dial("tcp", b.Host)
	if err != nil {
		log.Errorf("dial to %s error: %s", b.Host, err)
		// consume the client request,
		// or client will get forcely close connection error
		http.ReadRequest(bufio.NewReader(r))
		writeErrResponse(r, http.StatusBadGateway, err.Error())
		return
	}
	defer backend.Close()

	rb := bufio.NewReader(r)
	bb := bufio.NewReader(backend)
	for {
		req, err := http.ReadRequest(rb)
		if err != nil {
			if err != io.EOF {
				log.Errorf("read http request error: %s", err)
			}
			return
		}

		addr := r.RemoteAddr().(*net.TCPAddr)

		req.Header.Add("X-Forwarded-For", addr.IP.String())
		req.Header.Add("X-Real-Ip", addr.IP.String())

		//log.Printf("%+v\n", req.Header)
		err = req.Write(backend)

		if req.Body != nil {
			req.Body.Close()
		}

		if err != nil {
			log.Errorf("write request to backend error: %s", err)
			writeErrResponse(r, http.StatusBadGateway, err.Error())
			return
		}

		res, err := http.ReadResponse(bb, req)
		if err != nil {
			if err != io.EOF {
				log.Errorf("read http response from backend error: %s", err)
			}
			writeErrResponse(r, http.StatusBadGateway, "gateway error")
			return
		}

		err = res.Write(r)

		if res.Body != nil {
			res.Body.Close()
		}

		if err != nil {
			log.Errorf("write response to client error: %s", err)
			return
		}
	}
}

package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/url"

	"github.com/go-yaml/yaml"
)

type conf struct {
	Listen         []string      `yaml:"listen"`
	Forward        []forward     `yaml:"forward"`
	Certificate    []certificate `yaml:"certificate"`
	DefaultBackend string        `yaml:"default_backend"`
}

type forward struct {
	SNI     string `yaml:"sni"`
	Backend string `yaml:"backend"`
}

type certificate struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

func loadConfig(f string) (c *conf, err error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return
	}
	c = &conf{}
	if err = yaml.Unmarshal(data, c); err != nil {
		return
	}
	return
}

func (f forward) match(sni string) bool {
	if f.SNI == sni {
		return true
	}
	return false
}

func (f forward) getBackend() *url.URL {
	u, err := url.Parse(f.Backend)
	if err != nil {
		log.Println(err)
		return nil
	}
	return u
}

func (c certificate) load() (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
	return cert, err
}

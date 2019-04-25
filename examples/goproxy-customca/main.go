package main

import (
	"flag"
	"log"
	"net/http"
	"crypto/tls"
	"sync"
	"fmt"
	"encoding/gob"
	"crypto/rsa"

    "path"
	"path/filepath"
	"os"

	"github.com/elazarl/goproxy"
)

type CertStore struct {
    certs map[string]*tls.Certificate
    mu sync.Mutex
}

var store *CertStore

func init() {
    gob.Register(rsa.PrivateKey{})
    gob.Register(tls.Certificate{})

    store = newCertStore()
    certsDir := "./certs"
    files := []string{}
    err := filepath.Walk(certsDir, func(f string, info os.FileInfo, err error) error {
        if filepath.Ext(f) == ".key" {
            return nil
        }
        if info.IsDir() {
            return nil
        }
        f = f[0:len(f)-len(filepath.Ext(f))]
        files = append(files, f)
        return nil
    })
    if err != nil {
        panic(err)
    }
    for _, f := range(files) {
        // path := filepath.Join(certsDir, f)
        cert, err := ReadCert(f)
        if err != nil {
            panic(err)
        }

        host := path.Base(f)
        // store.mu.Lock()
        store.certs[host] = cert
        // store.mu.Unlock()
    }
}

func (store *CertStore) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error){
    var cert *tls.Certificate
    var err error

    store.mu.Lock()
    defer store.mu.Unlock()
    cert, ok := store.certs[hostname]
    if !ok {
        fmt.Println("====== cert gen!", hostname)
        cert, err = gen()
        if err != nil {
            return nil, err
        }
        err = SerializeCert("./certs/" + hostname, cert)
        if err != nil {
            panic(err)
        }
        store.certs[hostname] = cert
    }
    return cert, nil
}

func newCertStore() *CertStore {
    store := &CertStore{}
    store.certs = make(map[string]*tls.Certificate)
    return store
}

func main() {
	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":4080", "proxy listen address")
	flag.Parse()
	setCA(caCert, caKey)
	proxy := goproxy.NewProxyHttpServer()
	proxy.CertStore = store
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}

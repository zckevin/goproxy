package main

import (
    "crypto/tls"
    "crypto/x509"
    "crypto/rsa"
    "io/ioutil"
)

func SerializeCert(path string, c *tls.Certificate) error {
    var err error

    err = ioutil.WriteFile(path + ".pub1", c.Certificate[0], 0644)
    if err != nil {
        return err
    }

    err = ioutil.WriteFile(path + ".pub2", c.Certificate[1], 0644)
    if err != nil {
        return err
    }

    privateKey := x509.MarshalPKCS1PrivateKey(c.PrivateKey.(*rsa.PrivateKey))
    err = ioutil.WriteFile(path + ".key", privateKey,  0644)
    if err != nil {
        return err
    }

    return nil
}

func ReadCert(path string) (*tls.Certificate, error) {
    pub1, err := ioutil.ReadFile(path + ".pub1")
    if err != nil {
        return nil, err
    }

    pub2, err := ioutil.ReadFile(path + ".pub2")
    if err != nil {
        return nil, err
    }

    raw, err := ioutil.ReadFile(path + ".key")
    if err != nil {
        return nil, err
    }

    key, err := x509.ParsePKCS1PrivateKey(raw)
    if err != nil {
        return nil, err
    }

    cert := &tls.Certificate {
        Certificate: [][]byte {pub1, pub2},
        PrivateKey: key,
    }
    return cert, nil
}


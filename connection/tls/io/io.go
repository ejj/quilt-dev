// Package io is used to ensure that the various pieces of code that interact
// with credentials read and write files to consistent locations.
package io

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kelda/kelda/connection/tls"
	"github.com/kelda/kelda/connection/tls/rsa"
	"github.com/kelda/kelda/util"
)

const (
	// MinionTLSDir is the directory in which the daemon will place TLS
	// credentials on cloud machines.
	MinionTLSDir = "/home/quilt/.quilt/tls"

	caCertFilename     = "certificate_authority.crt"
	caKeyFilename      = "certificate_authority.key"
	signedCertFilename = "quilt.crt"
	signedKeyFilename  = "quilt.key"
)

// File represents a file to be written to the filesystem.
type File struct {
	Path    string
	Content string
	Mode    os.FileMode
}

// ReadCredentials reads the TLS credentials contained within the directory.
func ReadCredentials(dir string) (tls.TLS, error) {
	caCert, err := util.ReadFile(caCertPath(dir))
	if err != nil {
		return tls.TLS{}, fmt.Errorf("read CA: %s", err)
	}

	signedCert, err := util.ReadFile(signedCertPath(dir))
	if err != nil {
		return tls.TLS{}, fmt.Errorf("read signed cert: %s", err)
	}

	signedKey, err := util.ReadFile(signedKeyPath(dir))
	if err != nil {
		return tls.TLS{}, fmt.Errorf("read signed key: %s", err)
	}

	return tls.New(caCert, signedCert, signedKey)
}

// ReadCA reads the certificate authority contained with the directory.
func ReadCA(dir string) (rsa.KeyPair, error) {
	caCert, err := util.ReadFile(caCertPath(dir))
	if err != nil {
		return rsa.KeyPair{}, fmt.Errorf("read cert: %s", err)
	}

	caKey, err := util.ReadFile(caKeyPath(dir))
	if err != nil {
		return rsa.KeyPair{}, fmt.Errorf("read key: %s", err)
	}

	return rsa.New(caCert, caKey)
}

// MinionFiles defines how files should be written to disk for installation on
// minions.
func MinionFiles(dir string, ca, signed rsa.KeyPair) []File {
	return []File{
		{Path: caCertPath(dir), Content: ca.CertString(), Mode: 0644},
		{Path: signedCertPath(dir), Content: signed.CertString(), Mode: 0644},
		{Path: signedKeyPath(dir), Content: signed.PrivateKeyString(),
			Mode: 0600},
	}
}

// DaemonFiles defines how files should be written to disk for use by the daemon.
func DaemonFiles(dir string, ca, signed rsa.KeyPair) []File {
	return append(MinionFiles(dir, ca, signed),
		File{Path: caKeyPath(dir), Content: ca.PrivateKeyString(), Mode: 0600})
}

// caCertPath defines where to write the certificate for the certificate authority.
func caCertPath(dir string) string {
	return filepath.Join(dir, caCertFilename)
}

// caKeyPath defines where to write the private key for the certificate authority.
func caKeyPath(dir string) string {
	return filepath.Join(dir, caKeyFilename)
}

// signedCertPath defines where to write the certificate for the signed certificate.
func signedCertPath(dir string) string {
	return filepath.Join(dir, signedCertFilename)
}

// signedKeyPath defines where to write the private key for the signed certificate.
func signedKeyPath(dir string) string {
	return filepath.Join(dir, signedKeyFilename)
}

//go:build !js

package znet

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"path"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
)

func CreateSSLCertificate(owner SSLCertificateOwner, years int) (caPEMBytes, certPEMBytes, certPrivKeyPEMBytes []byte, err error) {
	// set up our CA certificate
	subject := pkix.Name{
		Organization:  []string{owner.Organization},
		Country:       []string{owner.Country},
		Province:      []string{owner.Province},
		Locality:      []string{owner.Locality},
		StreetAddress: []string{owner.StreetAddress},
		PostalCode:    []string{owner.PostalCode},
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(2019),
		Subject:               subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(years, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject:      subject,
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(years, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
	return caPEM.Bytes(), certPEM.Bytes(), certPrivKeyPEM.Bytes(), nil
}

func CreateSSLCertificateTLSConfig(owner SSLCertificateOwner, years int) (serverTLSConf *tls.Config, clientTLSConf *tls.Config, err error) {
	caPEMBytes, certPEMBytes, certPrivKeyPEMBytes, err := CreateSSLCertificate(owner, years)
	serverCert, err := tls.X509KeyPair(certPEMBytes, certPrivKeyPEMBytes)
	if err != nil {
		return nil, nil, err
	}
	serverTLSConf = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(caPEMBytes)
	clientTLSConf = &tls.Config{
		RootCAs: certpool,
	}
	return
}

func CreateSSLCertificateToFilepair(owner SSLCertificateOwner, years int, certPath, privateKeyPath string) error {
	caPEMBytes, certPEMBytes, certPrivKeyPEMBytes, err := CreateSSLCertificate(owner, years)
	zdebug.Consume(caPEMBytes)

	err = zfile.WriteBytesToFile(certPEMBytes, certPath)
	if err != nil {
		return zlog.Error(err, "write cert", certPath)
	}
	err = zfile.WriteBytesToFile(certPrivKeyPEMBytes, privateKeyPath)
	if err != nil {
		return zlog.Error(err, "write priv key", privateKeyPath)
	}
	return nil
}

func (ZNetCalls) WriteSSLCertificate(info SSLCertificateInfo) error {
	dir, _ := path.Split(info.CertificatePath)
	zfile.MakeDirAllIfNotExists(dir)
	dir2, _ := path.Split(info.PrivateKeyPath)
	if dir2 != dir {
		zfile.MakeDirAllIfNotExists(dir2)
	}
	err := CreateSSLCertificateToFilepair(info.SSLCertificateOwner, info.YearsUntilExpiry, info.CertificatePath, info.PrivateKeyPath)
	if err != nil {
		return err
	}
	return nil
}

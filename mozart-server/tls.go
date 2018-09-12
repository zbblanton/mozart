package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
	"io"
)

func generateSha256(file string) string {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

//Only supports 1 IP.  No multiple hostname or IP support yet.
func generateSignedKeyPair(caCert string, caKey string, keyPairName string, ip string, path string) {
	// Load CA
	catls, err := tls.LoadX509KeyPair(defaultSSLPath+caCert, defaultSSLPath+caKey)
	if err != nil {
		panic(err)
	}
	ca, err := x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		panic(err)
	}
	// Prepare certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"Mozart"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		IPAddresses:  []net.IP{net.ParseIP(ip)},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &priv.PublicKey
	// Sign the certificate
	certB, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, catls.PrivateKey)
	if err != nil {
		panic(err)
	}
	// Public key
	certOut, err := os.Create(path + keyPairName + ".crt")
	if err != nil {
		panic(err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certB})
	certOut.Close()
	// Private key
	keyOut, err := os.OpenFile(path+keyPairName+".key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()
}

//Only supports 1 IP.  No multiple hostname or IP support yet.
func generateSignedKeyPairToMemory(caCert string, caKey string, keyPairName string, ip string) (publicKey, privateKey []byte){
	// Load CA
	catls, err := tls.LoadX509KeyPair(defaultSSLPath+caCert, defaultSSLPath+caKey)
	if err != nil {
		panic(err)
	}
	ca, err := x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		panic(err)
	}
	// Prepare certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"Mozart"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		IPAddresses:  []net.IP{net.ParseIP(ip)},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &priv.PublicKey
	// Sign the certificate
	certB, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, catls.PrivateKey)
	if err != nil {
		panic(err)
	}

	publicKey = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certB})
	privateKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return publicKey, privateKey
}

func generateCaKeyPair(caPairName string) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization: []string{"Mozart"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &priv.PublicKey
	caB, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		log.Println("create ca failed", err)
		return
	}
	// Public key
	certOut, err := os.Create(defaultSSLPath + caPairName + ".crt")
	if err != nil {
		panic(err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caB})
	certOut.Close()
	log.Print("written cert.pem\n")
	// Private key
	keyOut, err := os.OpenFile(defaultSSLPath+caPairName+".key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()
	log.Print("written key.pem\n")
}

//Only supports 1 IP.  No multiple hostname or IP support yet.
func signCSR(caCert string, caKey string, csr []byte, ip string) (cert []byte, err error) {
	//Load CA
	catls, err := tls.LoadX509KeyPair(config.CaCert, config.CaKey)
	if err != nil {
		return []byte{}, err
	}
	ca, err := x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		return []byte{}, err
	}
	//Prepare certificate
	newCert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"Mozart"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		IPAddresses:  []net.IP{net.ParseIP(ip)},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	//Parse the CSR
	clientCSR, err := x509.ParseCertificateRequest(csr)
	if err != nil {
		return []byte{}, err
	}

	//Sign the certificate
	certSigned, err := x509.CreateCertificate(rand.Reader, newCert, ca, clientCSR.PublicKey, catls.PrivateKey)

	//Public key
	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certSigned})

	return cert, nil
}

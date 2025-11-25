package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"

	"github.com/smallstep/certificates/api"
)

type SignRequest struct {
	CsrPEM  api.CertificateRequest `json:"csr"`
	Subject string                 `json:"subject"`
}

func NewSignRequest(subject string) (*ecdsa.PrivateKey, *SignRequest, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: subject,
		},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return nil, nil, err
	}
	cr, err := x509.ParseCertificateRequest(csr)
	if err != nil {
		return nil, nil, err
	}

	if err := cr.CheckSignature(); err != nil {
		return nil, nil, err
	}

	signReq := &SignRequest{
		CsrPEM:  api.CertificateRequest{CertificateRequest: cr},
		Subject: subject,
	}

	return privateKey, signReq, nil
}

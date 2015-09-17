package scamp

// import "errors"
// import "fmt"

import "bytes"
import "encoding/base64"

import "crypto"
import "crypto/rsa"
import "crypto/sha256"
import "crypto/rand"

var padding = []byte("=")

func VerifySHA256(rawPayload []byte, rsaPubKey *rsa.PublicKey, encodedSignature []byte, isURLEncoded bool) (valid bool, err error) {
	expectedSig, err := decodeUnpaddedBase64(encodedSignature, isURLEncoded)
	if err != nil {
		valid = false
		return
	}

	h := sha256.New()
	h.Write(rawPayload)
	digest := h.Sum(nil)

	err = rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, digest, expectedSig)
	if err != nil {
		valid = false
		return
	}

	return true, nil
}

func SignSHA256(rawPayload []byte, priv *rsa.PrivateKey) (signature []byte, err error) {
	// func SignPKCS1v15(rand io.Reader, priv *PrivateKey, hash crypto.Hash, hashed []byte) (s []byte, err error)
	return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, rawPayload)
}

func decodeUnpaddedBase64(incoming []byte, isURLEncoded bool) (decoded []byte, err error) {
	decoded = make([]byte, len(incoming))

	if isURLEncoded {
		if m := len(incoming) % 4; m != 0 {
			paddingBytes := bytes.Repeat(padding, 4-m)
			incoming = append(incoming, paddingBytes[:]...)
		}
		_,err = base64.URLEncoding.Decode(decoded, incoming)
	} else {
		decoded,err = base64.StdEncoding.DecodeString(string(incoming))
	}
	if(err != nil){
		return
	}

	return
}
package showbox

import (
	"bytes"
	"crypto/des"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
)

const (

	appKeyString = "moviebox"
	tripleDESKey = "123d6cedf626dy54233aa1w6"
	tripleDESIV = "wEiphTn!"

)

type envelope struct {

	AppKey string `json:"app_key"`
	Verify string `json:"verify"`
	EncryptData string `json:"encrypt_data"`

}

func sealRequest(payload any) (string, error) {

	plain, err := json.Marshal(payload)

	if err != nil {

		return "", err

	}

	ciphertext, err := encrypt(plain)

	if err != nil {

		return "", err

	}

	appKey := md5Hex([]byte(appKeyString))

	sealed := envelope{

		AppKey: appKey,
		Verify: md5Hex([]byte(appKey + tripleDESKey + ciphertext)),
		EncryptData: ciphertext,

	}

	body, err := json.Marshal(sealed)

	if err != nil {

		return "", err

	}

	return base64.StdEncoding.EncodeToString(body), nil

}

func encrypt(plain []byte) (string, error) {

	block, err := des.NewTripleDESCipher([]byte(tripleDESKey))

	if err != nil {

		return "", err

	}

	padded := pad(plain, block.BlockSize())

	out := make([]byte, len(padded))

	cipher.NewCBCEncrypter(block, []byte(tripleDESIV)).CryptBlocks(out, padded)

	return base64.StdEncoding.EncodeToString(out), nil

}

func pad(data []byte, size int) []byte {

	count := size - len(data)%size

	return append(data, bytes.Repeat([]byte{byte(count)}, count)...)

}

func md5Hex(data []byte) string {

	sum := md5.Sum(data)

	return hex.EncodeToString(sum[:])

}

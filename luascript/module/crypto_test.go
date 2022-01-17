package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestCrypto(t *testing.T) {
	luaCode := `
	print("crypto test")

	print("----------- base64 ------------")
	b64 = base64_encode("hello,world!")
	print(b64)
	print(base64_decode(b64))

	print("----------- crc32 ------------")
	print(crc32("hello,world!"))

	print("----------- hmac ------------")
	print(hmac("md5", "hello", "world"))
	print(hmac("sha1", "hello", "world", true))
	print(hmac("sha256", "hello", "world"))
	print(hmac("sha512", "hello", "world", true))

	print("---------- hex encode -----------")
	print(hex_encode("Hello"))

	print("----------- encrypt/decrypt ------------")

	-- aes-cbc  key length 16,24,32, iv length 16
	aescbc = encrypt("hello", "aes-cbc", "1234abcd1234abcd", "iviv12345678abcd", true)
	print(aescbc)
	print(decrypt(aescbc, "aes-cbc", "1234abcd1234abcd", "iviv12345678abcd", true))

	-- des-cbc key length 8, iv length 8
	descbc = encrypt("hello", "des-cbc", "1234abcd", "iviv1234")
	print(descbc)
	print(decrypt(descbc, "des-cbc", "1234abcd", "iviv1234"))

	-- des-cbc key length 8, iv length 8
	desecb = encrypt("hello", "des-ecb", "1234abcd", "iviv1234", true)
	print(desecb)
	print(decrypt(desecb, "des-ecb", "1234abcd", "iviv1234", true))
`
	l := lua.NewState()
	defer l.Close()

	RegisterCryptoFuncs(l)
	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}

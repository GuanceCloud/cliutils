package module

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestAliyunOSSExample(t *testing.T) {
	var luaCode = `
	ak, sk = "AccessKeyXXXXXXX", "SecretKeyXXXXXXXX"

	-- /BucketName/ObjectName/ObjectName..
	resource = "/leecha-test/test-directory/main.rs"

	date = os.date("!%a, %d %b %Y %X GMT")

	info = string.format("GET\n\n\n%s\n%s", date, resource)

	// hmac(mothod_str, data_str, key_str, true)
	sign = base64_encode(hmac("sha1", info, sk, true))

	auth = "OSS " .. ak .. ":" .. sign

	// http://OSS_EndPoint/ObejctName/ObejctName..
	response, err = http_request("GET", "http://leecha-test.oss-cn-shanghai.aliyuncs.com/test-directory/main.rs", {
		headers={
			["Date"]=date,
			["Authorization"]=auth
		}
	})

	if err == nil then
		print(response.status_code)
		print(response.body)
	else
		print(err)
	end
`
	l := lua.NewState()
	defer l.Close()

	RegisterHTTPFuncs(l)
	RegisterCryptoFuncs(l)
	if err := l.DoString(luaCode); err != nil {
		t.Error(err)
	}
}

package http

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	o := &SignOption{
		AuthorizationType: "ABC",
		SignHeaders:       []string{`Date`, `Content-MD5`, `Content-Type`},
		SK:                "sk-cba",
		AK:                "ak-123",
		Sign:              "",
	}

	r, err := http.NewRequest(`POST`, `http://1.2.3.4:1234/v1/api`, nil)
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	r.Header.Set(`Content-MD5`, fmt.Sprintf("%x", md5.Sum([]byte(`{}`))))
	r.Header.Set(`Content-Type`, `application/json`)

	o.Sign = o.SignReq(r)

	r.Header.Set(`Authorization`, fmt.Sprintf("%s %s:%s", o.AuthorizationType, o.AK, o.Sign))
	log.Printf("[debug] %+#v", o)

	o2 := &SignOption{
		AuthorizationType: "ABC",
		SignHeaders:       []string{`Date`, `Content-MD5`, `Content-Type`},
		SK:                "sk-cba",
		AK:                "ak-123",
	}

	if err := o2.ParseAuth(r); err != nil {
		t.Fatal(err)
	}

	if o2.Sign != o2.SignReq(r) {
		t.Fatalf("sign not match")
	}
	log.Printf("[debug] %+#v", o)
}

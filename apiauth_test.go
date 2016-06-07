package apiauth

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func base64md5(bytes []byte) string {
	sum := md5.Sum(bytes)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestCanonicalString(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com/some/path?x=1&b=2", nil)

	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Content-MD5", "WnNni3tnQAUFZDSkgFRwfQ==")
	req.Header.Add("Date", "Thu, 19 Mar 2015 19:24:24 GMT")

	want := "POST,text/plain,WnNni3tnQAUFZDSkgFRwfQ==,/some/path?x=1&b=2,Thu, 19 Mar 2015 19:24:24 GMT"
	require.Equal(t, want, CanonicalString(req))

	req.Method = "get"
	want = "GET,text/plain,WnNni3tnQAUFZDSkgFRwfQ==,/some/path?x=1&b=2,Thu, 19 Mar 2015 19:24:24 GMT"
	require.Equal(t, want, CanonicalString(req))
}

func TestCompute(t *testing.T) {
	canonicalString := "text/plain,WnNni3tnQAUFZDSkgFRwfQ==,/a?b=c,Thu, 19 Mar 2015 19:34:03 GMT"
	want := "cMgmUVsq4IiT7baALMM1euHnpCo="
	require.Equal(t, want, Compute(canonicalString, "secret"))
}

func TestDateForTime(t *testing.T) {
	chi, err := time.LoadLocation("America/Chicago")
	require.NoError(t, err)

	tm := time.Date(2015, time.March, 19, 14, 34, 03, 0, chi)
	require.Equal(t, "Thu, 19 Mar 2015 19:34:03 GMT", DateForTime(tm))
}

func TestDate(t *testing.T) {
	require.Equal(t, DateForTime(time.Now()), Date())
}

func TestSign_AuthorizationHeaderPresentx(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Date", Date())
	req.Header.Set("Authorization", "anything")

	err := Sign(req, "me", "key")
	require.Error(t, err)
}

func TestSign(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com/some/path?x=1&b=2", nil)

	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Content-MD5", "WnNni3tnQAUFZDSkgFRwfQ==")
	req.Header.Add("Date", "Thu, 19 Mar 2015 19:24:24 GMT")

	want := `APIAuth me:43DQKYwiMx3swEwa3raDq5tPxIo=`
	require.NoError(t, Sign(req, "me", "secret"))
	require.Equal(t, want, req.Header.Get("Authorization"))
}

func TestSign_AuthorizationHeaderPresent(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Date", Date())
	req.Header.Set("Authorization", "anything")

	err := Sign(req, "me", "key")
	require.Error(t, err)
}

func TestSign_GETNoDate(t *testing.T) {
	secretKey := "secret"

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	require.Error(t, Sign(req, "me", secretKey))
	require.Equal(t, "", req.Header.Get("Authorization"))

	req.Header.Set("date", Date())
	require.NoError(t, Sign(req, "me", secretKey))
}

func TestSign_WithBody(t *testing.T) {
	body := []byte(`post body`)
	req, _ := http.NewRequest("POST", "http://example.com", bytes.NewReader(body))

	req.Header.Set("Date", Date())
	require.Error(t, Sign(req, "me", "secret"))

	req.Header.Set("Content-Type", "text/plain")
	require.Error(t, Sign(req, "me", "secret"))

	req.Header.Set("Content-MD5", base64md5(body))
	require.NoError(t, Sign(req, "me", "secret"))
}

func TestParse(t *testing.T) {
	_, _, err := Parse("NotAPIAuth here")
	require.Error(t, err)

	_, _, err = Parse("APIAuth noseparator")
	require.Error(t, err)

	_, _, err = Parse("APIAuth nosig:")
	require.Error(t, err)

	_, _, err = Parse("APIAuth :noaccessID")
	require.Error(t, err)

	id, sig, err := Parse("APIAuth me:sig")
	require.Equal(t, "me", id)
	require.Equal(t, "sig", sig)
}

func TestVerify(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com/some/path?x=1&b=2", nil)

	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Content-MD5", "WnNni3tnQAUFZDSkgFRwfQ==")
	req.Header.Add("Date", "Thu, 19 Mar 2015 19:24:24 GMT")
	req.Header.Add("Authorization", `APIAuth me:43DQKYwiMx3swEwa3raDq5tPxIo=`)

	require.NoError(t, Verify(req, "secret"))
}

func TestVerify_BadAuthorizationHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Date", "Fri, 20 Mar 2015 19:37:40 GMT")

	require.Error(t, Verify(req, "secret"))

	req.Header.Set("Authorization", "something else")
	require.Error(t, Verify(req, "secret"))

	req.Header.Set("Authorization", "APIAuth nosig:")
	require.Error(t, Verify(req, "secret"))

	req.Header.Set("Authorization", "APIAuth :noaccessID")
	require.Error(t, Verify(req, "secret"))
}

func TestVerify_NoDate(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "APIAuth me:N7N1BXAWv6+RXos4vSAAd7D0XJY=")
	require.Error(t, Verify(req, "secret"))
}

func TestVerify_WithBody(t *testing.T) {
	body := []byte(`post body`)
	req, _ := http.NewRequest("POST", "http://example.com", bytes.NewReader(body))

	req.Header.Set("Date", "Fri, 20 Mar 2015 19:37:40 GMT")
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-MD5", "WnNni3tnQAUFZDSkgFRwfQ==")
	req.Header.Set("Authorization", "APIAuth me:M9OiyO1hNKuHot1ll6+MZZn4ymo=")
	require.NoError(t, Verify(req, "secret"))

	req.Header.Del("Content-Type")
	require.Error(t, Verify(req, "secret"))

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Del("Content-MD5")
	require.Error(t, Verify(req, "secret"))
}

func TestVerify_AcceptIncorrectMD5(t *testing.T) {
	body := []byte(`post body`)
	req, _ := http.NewRequest("POST", "http://example.com", bytes.NewReader(body))

	req.Header.Set("Date", "Fri, 20 Mar 2015 19:37:40 GMT")
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-MD5", "not actually an md5 but this lib does not care")
	req.Header.Set("Authorization", "APIAuth me:AOinPMj/pONpWXNIIgr58CecGQg=")

	require.NoError(t, Verify(req, "secret"))
}

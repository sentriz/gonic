package listenbrainz

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"testing"
)

func TestGetParamSignature(t *testing.T) {
	params := url.Values{}
	params.Add("ccc", "CCC")
	params.Add("bbb", "BBB")
	params.Add("aaa", "AAA")
	params.Add("ddd", "DDD")
	actual := getParamSignature(params, "secret")
	expected := fmt.Sprintf("%x", md5.Sum([]byte(
		"aaaAAAbbbBBBcccCCCdddDDDsecret",
	)))
	if actual != expected {
		t.Errorf("expected %x, got %s", expected, actual)
	}
}

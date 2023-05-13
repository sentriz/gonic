package lastfm

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetParamSignature(t *testing.T) {
	// arrange
	params := url.Values{
		"aaa": []string{"AAA"},
		"bbb": []string{"BBB"},
		"ccc": []string{"CCC"},
		"ddd": []string{"DDD"},
	}
	client := Client{"apiKey", "secret", nil}

	// act
	actual := client.getParamSignature(params)

	// assert
	expected := fmt.Sprintf("%x", md5.Sum([]byte(
		"aaaAAAbbbBBBcccCCCdddDDDsecret",
	)))
	assert.Equal(t, expected, actual)
}

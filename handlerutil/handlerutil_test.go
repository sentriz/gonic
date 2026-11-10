package handlerutil

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no query", "/rest/ping", "/rest/ping"},
		{"password", "/rest/ping?u=alice&p=secret&v=1.16", "/rest/ping?u=alice&p=***&v=1.16"},
		{"token and salt", "/rest/ping?u=alice&t=abc&s=xyz&v=1.16", "/rest/ping?u=alice&t=***&s=***&v=1.16"},
		{"API key", "/rest/ping?apiKey=secret&u=alice", "/rest/ping?apiKey=***&u=alice"},
		{"empty values", "/rest/ping?p=&t=&s=&apiKey=", "/rest/ping?p=***&t=***&s=***&apiKey=***"},
		{"repeated keys", "/rest/ping?p=first&p=second&u=alice&u=bob", "/rest/ping?p=***&p=***&u=alice&u=bob"},
		{"no value", "/rest/ping?u=alice&p", "/rest/ping?u=alice&p=***"},
		{"encoded key", "/rest/ping?%70=secret&u=alice", "/rest/ping?%70=***&u=alice"},
		{"malformed value", "/rest/ping?p=%zz&u=alice", "/rest/ping?p=***&u=alice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u, err := url.Parse(tt.in)
			require.NoError(t, err)
			original := *u
			require.Equal(t, tt.want, redactURL(u))
			require.Equal(t, original, *u)
		})
	}
}

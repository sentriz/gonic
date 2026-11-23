package ctrladmin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/deps"
)

func TestParseGroups(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"admins", []string{"admins"}},
		{"admins,users", []string{"admins", "users"}},
		{"admins|users", []string{"admins", "users"}},
		{" admins , users ", []string{"admins", "users"}},
		{"admins| users,editors ", []string{"admins", "users", "editors"}},
	}
	for _, tc := range cases {
		got := parseGroups(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("parseGroups(%q) len=%d want %d (%v)", tc.in, len(got), len(tc.want), got)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("parseGroups(%q)[%d]=%q want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestAuthenticateProxyUser(t *testing.T) {
	t.Parallel()
	dbc := openTestDB(t)
	c := &Controller{
		dbc:              dbc,
		authHeader:       "Remote-User",
		authGroupsHeader: "Remote-Groups",
		authAdminGroup:   "gonic-admins",
	}

	t.Run("disabled when header empty", func(t *testing.T) {
		t.Parallel()
		cDisabled := &Controller{dbc: dbc}
		user, err := cDisabled.authenticateProxyUser(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil || user != nil {
			t.Fatalf("got user=%v err=%v, want nil", user, err)
		}
	})

	t.Run("absent header falls through", func(t *testing.T) {
		t.Parallel()
		user, err := c.authenticateProxyUser(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil || user != nil {
			t.Fatalf("got user=%v err=%v, want nil", user, err)
		}
	})

	t.Run("creates user and syncs admin", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Remote-User", "alice")
		r.Header.Set("Remote-Groups", "users,gonic-admins")

		user, err := c.authenticateProxyUser(r)
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		if user == nil || user.Name != "alice" {
			t.Fatalf("got %+v", user)
		}
		if !user.IsAdmin {
			t.Fatal("expected admin from group membership")
		}
		if user.Password == "" {
			t.Fatal("expected generated password for subsonic clients")
		}

		// second request without admin group demotes
		r2 := httptest.NewRequest(http.MethodGet, "/", nil)
		r2.Header.Set("Remote-User", "alice")
		r2.Header.Set("Remote-Groups", "users")
		user2, err := c.authenticateProxyUser(r2)
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		if user2.IsAdmin {
			t.Fatal("expected admin demotion when group removed")
		}
		if user2.ID != user.ID {
			t.Fatal("expected same user row")
		}
	})
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dbc, err := db.NewMock(deps.DBDriverOptions())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = dbc.Close() })
	if err := dbc.Migrate(db.MigrationContext{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	dbc.LogMode(false)
	return dbc
}

func TestChangeUsernameDisabledWithProxyAuth(t *testing.T) {
	t.Parallel()
	c := &Controller{authHeader: "Remote-User"}
	r := httptest.NewRequest(http.MethodGet, "/admin/change_username?user=alice", nil)

	for _, resp := range []*Response{c.ServeChangeUsername(r), c.ServeChangeUsernameDo(r)} {
		if resp.redirect != "/admin/home" {
			t.Fatalf("redirect=%q want /admin/home", resp.redirect)
		}
		if len(resp.flashW) == 0 {
			t.Fatal("expected warning flash")
		}
	}
}

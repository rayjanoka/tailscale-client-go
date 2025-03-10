package tailscale_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tailscale/hujson"

	"github.com/davidsbond/tailscale-client-go/tailscale"
)

var (
	//go:embed testdata/acl.json
	jsonACL []byte
	//go:embed testdata/acl.hujson
	huJSONACL []byte
)

func TestACL_Unmarshal(t *testing.T) {
	t.Parallel()

	tt := []struct {
		Name          string
		ACLContent    []byte
		Expected      tailscale.ACL
		UnmarshalFunc func(data []byte, v interface{}) error
	}{
		{
			Name:          "It should handle JSON ACLs",
			ACLContent:    jsonACL,
			UnmarshalFunc: json.Unmarshal,
			Expected: tailscale.ACL{
				ACLs: []tailscale.ACLEntry{
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"autogroup:members"},
						Destination: []string{"autogroup:self:*"},
						Protocol:    "",
					},
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"group:dev"},
						Destination: []string{"tag:dev:*"},
						Protocol:    "",
					},
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"group:devops"},
						Destination: []string{"tag:prod:*"},
						Protocol:    "",
					},
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"autogroup:members"},
						Destination: []string{"tag:monitoring:80,443"},
						Protocol:    "",
					},
				},
				Groups: map[string][]string{
					"group:dev":    {"alice@example.com", "bob@example.com"},
					"group:devops": {"carl@example.com"},
				},
				Hosts: map[string]string(nil),
				TagOwners: map[string][]string{
					"tag:dev":        {"group:devops"},
					"tag:monitoring": {"group:devops"},
					"tag:prod":       {"group:devops"},
				},
				DERPMap: (*tailscale.ACLDERPMap)(nil),
				Tests: []tailscale.ACLTest{
					{
						User:        "",
						Allow:       []string(nil),
						Deny:        []string(nil),
						Source:      "carl@example.com",
						Accept:      []string{"tag:prod:80"},
					},
					{
						User:        "",
						Allow:       []string(nil),
						Deny:        []string{"tag:prod:80"},
						Source:      "alice@example.com",
						Accept:      []string{"tag:dev:80"}},
				},
			},
		},
		{
			Name:          "It should handle HuJSON ACLs",
			ACLContent:    huJSONACL,
			UnmarshalFunc: hujson.Unmarshal,
			Expected: tailscale.ACL{
				ACLs: []tailscale.ACLEntry{
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"autogroup:members"},
						Destination: []string{"autogroup:self:*"},
						Protocol:    "",
					},
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"group:dev"},
						Destination: []string{"tag:dev:*"},
						Protocol:    "",
					},
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"group:devops"},
						Destination: []string{"tag:prod:*"},
						Protocol:    "",
					},
					{
						Action:      "accept",
						Ports:       []string(nil),
						Users:       []string(nil),
						Source:      []string{"autogroup:members"},
						Destination: []string{"tag:monitoring:80,443"},
						Protocol:    "",
					},
				},
				Groups: map[string][]string{
					"group:dev":    {"alice@example.com", "bob@example.com"},
					"group:devops": {"carl@example.com"},
				},
				Hosts: map[string]string(nil),
				TagOwners: map[string][]string{
					"tag:dev":        {"group:devops"},
					"tag:monitoring": {"group:devops"},
					"tag:prod":       {"group:devops"},
				},
				DERPMap: (*tailscale.ACLDERPMap)(nil),
				Tests: []tailscale.ACLTest{
					{
						User:        "",
						Allow:       []string(nil),
						Deny:        []string(nil),
						Source:      "carl@example.com",
						Accept:      []string{"tag:prod:80"},
					},
					{
						User:        "",
						Allow:       []string(nil),
						Deny:        []string{"tag:prod:80"},
						Source:      "alice@example.com",
						Accept:      []string{"tag:dev:80"}},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			var actual tailscale.ACL

			assert.NoError(t, tc.UnmarshalFunc(tc.ACLContent, &actual))
			assert.EqualValues(t, tc.Expected, actual)
		})
	}
}

func TestClient_SetACL(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK
	expectedACL := tailscale.ACL{
		ACLs: []tailscale.ACLEntry{
			{
				Action: "accept",
				Ports:  []string{"*:*"},
				Users:  []string{"*"},
			},
		},
		TagOwners: map[string][]string{
			"tag:example": {"group:example"},
		},
		Hosts: map[string]string{
			"example-host-1": "100.100.100.100",
			"example-host-2": "100.100.101.100/24",
		},
		Groups: map[string][]string{
			"group:example": {
				"user1@example.com",
				"user2@example.com",
			},
		},
		Tests: []tailscale.ACLTest{
			{
				User:  "user1@example.com",
				Allow: []string{"example-host-1:22", "example-host-2:80"},
				Deny:  []string{"exapmle-host-2:100"},
			},
			{
				User:  "user2@example.com",
				Allow: []string{"100.60.3.4:22"},
			},
		},
	}

	assert.NoError(t, client.SetACL(context.Background(), expectedACL))
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/acl", server.Path)

	var actualACL tailscale.ACL
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &actualACL))
	assert.EqualValues(t, expectedACL, actualACL)
}

func TestClient_ACL(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)

	server.ResponseCode = http.StatusOK
	server.ResponseBody = &tailscale.ACL{
		ACLs: []tailscale.ACLEntry{
			{
				Action: "accept",
				Ports:  []string{"*:*"},
				Users:  []string{"*"},
			},
		},
		TagOwners: map[string][]string{
			"tag:example": {"group:example"},
		},
		Hosts: map[string]string{
			"example-host-1": "100.100.100.100",
			"example-host-2": "100.100.101.100/24",
		},
		Groups: map[string][]string{
			"group:example": {
				"user1@example.com",
				"user2@example.com",
			},
		},
		Tests: []tailscale.ACLTest{
			{
				User:  "user1@example.com",
				Allow: []string{"example-host-1:22", "example-host-2:80"},
				Deny:  []string{"exapmle-host-2:100"},
			},
			{
				User:  "user2@example.com",
				Allow: []string{"100.60.3.4:22"},
			},
		},
	}

	acl, err := client.ACL(context.Background())
	assert.NoError(t, err)
	assert.EqualValues(t, server.ResponseBody, acl)
	assert.EqualValues(t, http.MethodGet, server.Method)
	assert.EqualValues(t, "/api/v2/tailnet/example.com/acl", server.Path)
}

func TestClient_SetDeviceSubnetRoutes(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	const deviceID = "test"
	routes := []string{"127.0.0.1"}

	assert.NoError(t, client.SetDeviceSubnetRoutes(context.Background(), deviceID, routes))
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/device/test/routes", server.Path)

	body := make(map[string][]string)
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &body))
	assert.EqualValues(t, routes, body["routes"])
}

func TestClient_Devices(t *testing.T) {
	t.Parallel()

	expectedDevices := map[string][]tailscale.Device{
		"devices": {
			{
				Addresses:         []string{"127.0.0.1"},
				Name:              "test",
				ID:                "test",
				Authorized:        true,
				KeyExpiryDisabled: true,
				User:              "test@example.com",
				Tags: []string{
					"tag:value",
				},
				BlocksIncomingConnections: false,
				ClientVersion:             "1.22.1",
				Created:                   time.Date(2022, 2, 10, 11, 50, 23, 0, time.UTC),
				Expires:                   time.Date(2022, 8, 9, 11, 50, 23, 0, time.UTC),
				Hostname:                  "test",
				IsExternal:                false,
				LastSeen:                  time.Date(2022, 3, 9, 20, 3, 42, 0, time.UTC),
				MachineKey:                "mkey:test",
				NodeKey:                   "nodekey:test",
				OS:                        "windows",
				UpdateAvailable:           true,
			},
		},
	}

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK
	server.ResponseBody = expectedDevices

	actualDevices, err := client.Devices(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/devices", server.Path)
	assert.EqualValues(t, expectedDevices["devices"], actualDevices)
}

func TestClient_DeleteDevice(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK
	ctx := context.Background()

	deviceID := "deviceTestId"
	assert.NoError(t, client.DeleteDevice(ctx, deviceID))
	assert.Equal(t, http.MethodDelete, server.Method)
	assert.Equal(t, "/api/v2/device/deviceTestId", server.Path)
}

func TestClient_DeviceSubnetRoutes(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK
	server.ResponseBody = &tailscale.DeviceRoutes{
		Advertised: []string{"127.0.0.1"},
		Enabled:    []string{"127.0.0.1"},
	}

	const deviceID = "test"

	routes, err := client.DeviceSubnetRoutes(context.Background(), deviceID)
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, server.Method)
	assert.Equal(t, "/api/v2/device/test/routes", server.Path)
	assert.Equal(t, server.ResponseBody, routes)
}

func TestClient_DNSNameservers(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	expectedNameservers := map[string][]string{
		"dns": {"127.0.0.1"},
	}

	server.ResponseBody = expectedNameservers
	nameservers, err := client.DNSNameservers(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/dns/nameservers", server.Path)
	assert.Equal(t, expectedNameservers["dns"], nameservers)
}

func TestClient_DNSPreferences(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK
	server.ResponseBody = &tailscale.DNSPreferences{
		MagicDNS: true,
	}

	preferences, err := client.DNSPreferences(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/dns/preferences", server.Path)
	assert.Equal(t, server.ResponseBody, preferences)
}

func TestClient_DNSSearchPaths(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	expectedPaths := map[string][]string{
		"searchPaths": {"test"},
	}

	server.ResponseBody = expectedPaths

	paths, err := client.DNSSearchPaths(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/dns/searchpaths", server.Path)
	assert.Equal(t, expectedPaths["searchPaths"], paths)
}

func TestClient_SetDNSNameservers(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	nameservers := []string{"127.0.0.1"}

	assert.NoError(t, client.SetDNSNameservers(context.Background(), nameservers))
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/dns/nameservers", server.Path)

	body := make(map[string][]string)
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &body))
	assert.EqualValues(t, nameservers, body["dns"])
}

func TestClient_SetDNSPreferences(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	preferences := tailscale.DNSPreferences{
		MagicDNS: true,
	}

	assert.NoError(t, client.SetDNSPreferences(context.Background(), preferences))
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/dns/preferences", server.Path)

	var body tailscale.DNSPreferences
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &body))
	assert.EqualValues(t, preferences, body)
}

func TestClient_SetDNSSearchPaths(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	paths := []string{"test"}

	assert.NoError(t, client.SetDNSSearchPaths(context.Background(), paths))
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/dns/searchpaths", server.Path)

	body := make(map[string][]string)
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &body))
	assert.EqualValues(t, paths, body["searchPaths"])
}

func TestClient_AuthorizeDevice(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	const deviceID = "test"

	assert.NoError(t, client.AuthorizeDevice(context.Background(), deviceID))
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/device/test/authorized", server.Path)

	body := make(map[string]bool)
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &body))
	assert.EqualValues(t, true, body["authorized"])
}

func TestClient_CreateKey(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	capabilities := tailscale.KeyCapabilities{}
	capabilities.Devices.Create.Ephemeral = true
	capabilities.Devices.Create.Reusable = true

	expected := tailscale.Key{
		ID:           "test",
		Key:          "thisisatestkey",
		Created:      time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Expires:      time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Capabilities: capabilities,
	}

	server.ResponseBody = expected

	actual, err := client.CreateKey(context.Background(), capabilities)
	assert.NoError(t, err)
	assert.EqualValues(t, expected, actual)
	assert.Equal(t, http.MethodPost, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/keys", server.Path)
}

func TestClient_GetKey(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	capabilities := tailscale.KeyCapabilities{}
	capabilities.Devices.Create.Ephemeral = true
	capabilities.Devices.Create.Reusable = true

	expected := tailscale.Key{
		ID:           "test",
		Created:      time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Expires:      time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Capabilities: capabilities,
	}

	server.ResponseBody = expected

	actual, err := client.GetKey(context.Background(), expected.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, expected, actual)
	assert.Equal(t, http.MethodGet, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/keys/"+expected.ID, server.Path)
}

func TestClient_DeleteKey(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	const keyID = "test"

	assert.NoError(t, client.DeleteKey(context.Background(), keyID))
	assert.Equal(t, http.MethodDelete, server.Method)
	assert.Equal(t, "/api/v2/tailnet/example.com/keys/"+keyID, server.Path)
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusNotFound
	server.ResponseBody = tailscale.APIError{Message: "error"}

	_, err := client.GetKey(context.Background(), "test")
	assert.True(t, tailscale.IsNotFound(err))
}

func TestClient_SetDeviceTags(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	const deviceID = "test"
	tags := []string{"a:b", "b:c"}

	assert.NoError(t, client.SetDeviceTags(context.Background(), deviceID, tags))
	assert.EqualValues(t, http.MethodPost, server.Method)
	assert.EqualValues(t, "/api/v2/device/"+deviceID+"/tags", server.Path)

	body := make(map[string][]string)
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &body))
	assert.EqualValues(t, tags, body["tags"])
}

func TestClient_SetDeviceKey(t *testing.T) {
	t.Parallel()

	client, server := NewTestHarness(t)
	server.ResponseCode = http.StatusOK

	const deviceID = "test"
	expected := tailscale.DeviceKey{
		KeyExpiryDisabled: true,
		Preauthorized:     true,
	}

	assert.NoError(t, client.SetDeviceKey(context.Background(), deviceID, expected))

	assert.EqualValues(t, http.MethodPost, server.Method)
	assert.EqualValues(t, "/api/v2/device/"+deviceID+"/key", server.Path)

	var actual tailscale.DeviceKey
	assert.NoError(t, json.Unmarshal(server.Body.Bytes(), &actual))
	assert.EqualValues(t, expected, actual)

}

func TestErrorData(t *testing.T) {
	t.Parallel()

	t.Run("It should return the data element from a valid error", func(t *testing.T) {
		expected := tailscale.APIError{
			Data: []tailscale.APIErrorData{
				{
					User: "user1@example.com",
					Errors: []string{
						"address \"user2@example.com:400\": want: Accept, got: Drop",
					},
				},
			},
		}

		actual := tailscale.ErrorData(expected)
		assert.EqualValues(t, expected.Data, actual)
	})

	t.Run("It should return an empty slice for any other error", func(t *testing.T) {
		assert.Empty(t, tailscale.ErrorData(io.EOF))
	})
}

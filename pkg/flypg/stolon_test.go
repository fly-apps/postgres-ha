package flypg

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateKeeperID(t *testing.T) {
	cases := map[string]string{
		"fdaa:0:1:a7b:6b:0:1e7a:2":  "6b01e7a2",
		"fdaa:0:1a:a7b:7b:0:21e9:2": "7b021e92",
	}

	for ip, expected := range cases {
		privateIP := net.ParseIP(ip)

		assert.Equal(t, expected, keeperUID(privateIP))
	}
}

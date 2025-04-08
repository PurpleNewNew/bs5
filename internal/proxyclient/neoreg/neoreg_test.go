package neoreg

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/url"
	"testing"
)

func TestNewConfFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantErr  bool
		checkKey bool
		protocol string
	}{
		{
			name:     "valid http url",
			url:      "neoreg://password@example.com/path",
			wantErr:  false,
			checkKey: true,
			protocol: "http",
		},
		{
			name:     "valid https url",
			url:      "neoregs://password@example.com/path",
			wantErr:  false,
			checkKey: true,
			protocol: "https",
		},
		{
			name:    "url without auth",
			url:     "neoreg://example.com/path",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			url:     "http://password@example.com/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatal(err)
			}

			conf, err := NewConfFromURL(proxyURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.checkKey {
					if len(conf.Key) != minKeyLen {
						t.Errorf("Key length = %v, want %v", len(conf.Key), minKeyLen)
					}
					if conf.EncodeMap == nil || conf.DecodeMap == nil {
						t.Error("Maps not initialized")
					}
					if conf.blvOffset == 0 {
						t.Error("BLV offset not initialized")
					}
				}
				if conf.Protocol != tt.protocol {
					t.Errorf("Protocol = %v, want %v", conf.Protocol, tt.protocol)
				}
			}
		})
	}
}

func TestNeoregClientDial(t *testing.T) {
	// Create test client
	proxyURL, _ := url.Parse("neoreg://password@127.0.0.1:8089/tunnel.jsp")
	conf, err := NewConfFromURL(proxyURL)
	if err != nil {
		t.Fatal(err)
	}

	client := &NeoregClient{
		Proxy: proxyURL,
		Conf:  conf,
	}

	// Test connection
	conn, err := client.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// Verify connection type
	if _, ok := conn.(*neoregConn); !ok {
		t.Error("Expected neoregConn type")
	}
	_, err = conn.Write([]byte("restet"))
	if err != nil {
		return
	}
}

func TestBlvEncoding(t *testing.T) {
	// 使用固定的随机种子以获得确定的结果
	rand.Seed(0)

	testCases := []struct {
		name   string
		info   map[int][]byte
		offset int32
	}{
		{
			name: "basic test",
			info: map[int][]byte{
				cmdCommand: []byte("CONNECT"),
				cmdMark:    []byte("test"),
				cmdIP:      []byte("127.0.0.1"),
				cmdPort:    []byte("8080"),
			},
			offset: 288179968,
		},
		{
			name: "empty data",
			info: map[int][]byte{
				cmdCommand: []byte("READ"),
				cmdMark:    []byte("test"),
			},
			offset: 288179968,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 编码
			encoded := blvEncode(tc.info, tc.offset)
			decoded := blvDecode(encoded, tc.offset)

			// 检查命令字段
			for k, v := range tc.info {
				if !bytes.Equal(decoded[k], v) {
					t.Errorf("Field %d: got %v, want %v", k, decoded[k], v)
				}
			}

			// 检查填充字段是否存在且非空
			if v, ok := decoded[0]; !ok || len(v) == 0 {
				t.Error("Missing or empty head padding")
			}
			if v, ok := decoded[39]; !ok || len(v) == 0 {
				t.Error("Missing or empty tail padding")
			}

			// 检查编解码一致性
			reEncoded := blvEncode(decoded, tc.offset)
			reDecoded := blvDecode(reEncoded, tc.offset)

			// 确保两次解码结果一致
			for k, v := range decoded {
				if !bytes.Equal(reDecoded[k], v) {
					t.Errorf("Inconsistent encoding/decoding for field %d", k)
				}
			}
		})
	}
}

func TestPythonCompatibility(t *testing.T) {
	pythonKey := "password"
	rng := NewNeoregRand(pythonKey)
	println(rng.mt.GetRandBits(31).Uint64())
	char := []rune(BASE64CHARS)
	rng.Base64Chars(char)
	fmt.Println(string(char))
}

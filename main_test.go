package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

// captureOutput은 stdout과 stderr을 별도로 캡처합니다.
func captureOutput(f func()) (string, string) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldStderr := os.Stderr
	re, we, _ := os.Pipe()
	os.Stderr = we

	f()

	w.Close()
	os.Stdout = oldStdout
	we.Close()
	os.Stderr = oldStderr

	var outBuf strings.Builder
	io.Copy(&outBuf, r)

	var errBuf strings.Builder
	io.Copy(&errBuf, re)

	return outBuf.String(), errBuf.String()
}

// TestGetMetaDatas는 이전과 동일하게 API 통신을 테스트합니다.
func TestGetMetaDatas(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"hooks":   []string{"192.168.1.1"},
				"actions": []string{"172.16.0.1"},
			})
		}))
		defer server.Close()

		originalURL := GITHUB_META_URL
		GITHUB_META_URL = server.URL
		defer func() { GITHUB_META_URL = originalURL }()

		meta, err := getMetaDatas()
		assert.NoError(t, err)
		assert.NotNil(t, meta)
		assert.Contains(t, meta, "hooks")
	})

	t.Run("HTTPError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		originalURL := GITHUB_META_URL
		GITHUB_META_URL = server.URL
		defer func() { GITHUB_META_URL = originalURL }()

		_, err := getMetaDatas()
		assert.Error(t, err)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"hooks": ["1.1.1.1"],}`)
		}))
		defer server.Close()

		originalURL := GITHUB_META_URL
		GITHUB_META_URL = server.URL
		defer func() { GITHUB_META_URL = originalURL }()

		_, err := getMetaDatas()
		assert.Error(t, err)
	})
}

// TestSaveToFile은 새로운 파일 이름 규칙과 포맷을 검증합니다.
func TestSaveToFile(t *testing.T) {
	tempDir := t.TempDir()
	// 다양한 케이스를 포함하는 테스트 데이터
	mockRuleSets := []FirewallRuleSet{
		{Category: "actions", Port: 443, IPs: []interface{}{"172.16.0.1"}},
		{Category: "git", Port: 22, IPs: []interface{}{"192.168.1.1"}},
		{Category: "git", Port: 443, IPs: []interface{}{"192.168.1.1"}}, // 동일 타입, 다른 포트
	}

	t.Run("JSONBackend", func(t *testing.T) {
		err := saveToFile(mockRuleSets, tempDir, "json")
		assert.NoError(t, err)

		filePath := filepath.Join(tempDir, "github_meta.json")
		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		var decodedData map[string][]interface{}
		json.Unmarshal(content, &decodedData)

		assert.Contains(t, decodedData, "actions:443")
		assert.Contains(t, decodedData, "git:22")
		assert.Contains(t, decodedData, "git:443")
		assert.Equal(t, []interface{}{"172.16.0.1"}, decodedData["actions:443"])
	})

	t.Run("IPTablesBackend", func(t *testing.T) {
		err := saveToFile(mockRuleSets, tempDir, "iptables")
		assert.NoError(t, err)

		// actions_443.txt 파일 검증
		content, err := os.ReadFile(filepath.Join(tempDir, "actions_443.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "iptables -A INPUT -p tcp -s 172.16.0.1 --dport 443 -j ACCEPT", string(content))

		// git_22.txt 파일 검증
		content, err = os.ReadFile(filepath.Join(tempDir, "git_22.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "iptables -A INPUT -p tcp -s 192.168.1.1 --dport 22 -j ACCEPT", string(content))

		// git_443.txt 파일 검증
		content, err = os.ReadFile(filepath.Join(tempDir, "git_443.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "iptables -A INPUT -p tcp -s 192.168.1.1 --dport 443 -j ACCEPT", string(content))
	})

	t.Run("NetshBackend", func(t *testing.T) {
		err := saveToFile(mockRuleSets, tempDir, "netsh")
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tempDir, "actions_443.txt"))
		assert.NoError(t, err)
		assert.Equal(t, `netsh advfirewall firewall add rule name="Allow GitHub actions port 443" dir=in action=allow protocol=TCP remoteip=172.16.0.1 localport=443`, string(content))
	})

	t.Run("UFWBackend", func(t *testing.T) {
		err := saveToFile(mockRuleSets, tempDir, "ufw")
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tempDir, "git_22.txt"))
		assert.NoError(t, err)
		assert.Equal(t, `ufw allow from 192.168.1.1 to any port 22 comment 'Allow GitHub git'`, string(content))
	})

	t.Run("DefaultBackend", func(t *testing.T) {
		err := saveToFile(mockRuleSets, tempDir, "") // Default backend
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tempDir, "actions_443.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "172.16.0.1", string(content))
	})

	t.Run("CreateDirectory", func(t *testing.T) {
		newDir := filepath.Join(tempDir, "new_dir_test")
		err := saveToFile(mockRuleSets, newDir, "json")
		assert.NoError(t, err)
		assert.DirExists(t, newDir)
	})
}

// TestPrintToStdout은 새로운 출력 포맷을 검증합니다.
func TestPrintToStdout(t *testing.T) {
	color.NoColor = true // Disable color output for tests
	mockRuleSets := []FirewallRuleSet{
		{Category: "actions", Port: 443, IPs: []interface{}{"172.16.0.1"}},
		{Category: "git", Port: 22, IPs: []interface{}{"192.168.1.1"}},
	}

	runTest := func(f func()) string {
		stdout, stderr := captureOutput(f)
		return stdout + stderr
	}

	t.Run("IPTablesBackend", func(t *testing.T) {
		output := runTest(func() {
			err := printToStdout(mockRuleSets, "iptables")
			assert.NoError(t, err)
		})
		assert.Contains(t, output, "# Category: actions, Port: 443 (1 entries)")
		assert.Contains(t, output, "iptables -A INPUT -p tcp -s 172.16.0.1 --dport 443 -j ACCEPT")
		assert.Contains(t, output, "# Category: git, Port: 22 (1 entries)")
		assert.Contains(t, output, "iptables -A INPUT -p tcp -s 192.168.1.1 --dport 22 -j ACCEPT")
	})

	t.Run("JSONBackend", func(t *testing.T) {
		stdout, _ := captureOutput(func() {
			err := printToStdout(mockRuleSets, "json")
			assert.NoError(t, err)
		})

		var decodedData map[string][]interface{}
		err := json.Unmarshal([]byte(stdout), &decodedData)
		assert.NoError(t, err)
		assert.Contains(t, decodedData, "actions:443")
		assert.Contains(t, decodedData, "git:22")
	})
}

// TestListTypesFlag는 --list-types 플래그의 동작을 테스트합니다.
func TestListTypesFlag(t *testing.T) {
	color.NoColor = true // 테스트에서 색상 출력을 비활성화합니다.

	// 1. 모의 서버 설정
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hooks":                              []string{"192.168.1.1"},
			"git":                                []string{"192.168.1.2"},
			"actions":                            []string{"172.16.0.1"},
			"verifiable_password_authentication": true, // 이 키는 IP 목록이 아니므로 무시되어야 합니다.
		})
	}))
	defer server.Close()
	originalURL := GITHUB_META_URL
	GITHUB_META_URL = server.URL
	defer func() { GITHUB_META_URL = originalURL }()

	// 2. 플래그 및 인자 설정
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"github-meta", "--list-types"}

	// 플래그 파싱 상태를 초기화하여 main 함수를 안전하게 호출합니다.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// 3. main 함수의 출력을 캡처합니다.
	stdout, stderr := captureOutput(main)

	// 4. 결과 검증
	assert.Contains(t, stderr, "Fetching latest IP data from GitHub...")
	assert.Contains(t, stderr, "Successfully fetched data!")

	expectedStdout := "Available IP types from GitHub Meta API:\n" +
		"- actions (default port: 443)\n" +
		"- git (default port: 22)\n" +
		"- hooks (default port: 443)\n"
	assert.Equal(t, expectedStdout, stdout)
}
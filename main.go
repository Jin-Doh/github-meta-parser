package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

const (
	VERSION            = "0.4.0"
	SUPPORTED_BACKENDS = "iptables, netsh, ufw, json"
)

var (
	GITHUB_META_URL = "https://api.github.com/meta"
	AllowedTypes    = []string{}
	// 대부분의 GitHub 서비스는 443 포트를 사용합니다.
	// SSH 관련 명확한 키에 대해서만 22 포트를 기본값으로 설정합니다.
	defaultPorts = map[string]int{
		"git":      22,
		"ssh_keys": 22,
	}
)

// getDefaultPort는 카테고리에 대한 기본 포트를 반환합니다.
// 명시적인 기본값이 없으면 443을 반환합니다.
func getDefaultPort(category string) int {
	if p, ok := defaultPorts[category]; ok {
		return p
	}
	return 443
}

// FirewallRuleSet은 방화벽 규칙 생성을 위한 정보를 담는 구조체입니다.
type FirewallRuleSet struct {
	Category string
	Port     int
	IPs      []interface{}
}

// GetMetaDatas 함수는 `GITHUB_META_URL`에 요청을 보내 JSON 응답을 파싱하여 메타 데이터를 반환합니다.
func getMetaDatas() (map[string]interface{}, error) {
	fmt.Fprintln(os.Stderr, "Fetching latest IP data from GitHub...")
	resp, err := http.Get(GITHUB_META_URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch metadata: %s", resp.Status)
	}

	var meta map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, color.GreenString("Successfully fetched data!"))
	return meta, nil
}

// saveToFile 함수는 메타 데이터를 지정된 위치에 저장합니다.
func saveToFile(ruleSets []FirewallRuleSet, outputDir string, backend string) error {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		fmt.Printf("Output directory does not exist. Creating: %s\n", outputDir)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// JSON 백엔드의 경우, 모든 데이터를 하나의 파일에 저장
	if backend == "json" {
		outputFile := filepath.Join(outputDir, "github_meta.json")
		// JSON 출력용으로 데이터를 재구성합니다.
		jsonData := make(map[string]interface{})
		for _, rs := range ruleSets {
			key := fmt.Sprintf("%s:%d", rs.Category, rs.Port)
			jsonData[key] = rs.IPs
		}
		jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %v", err)
		}
		if err := os.WriteFile(outputFile, jsonBytes, 0644); err != nil {
			return fmt.Errorf("failed to write to file %s: %v", outputFile, err)
		}
		color.Green("Saved all data to %s\n", outputFile)
		return nil
	}

	for _, rs := range ruleSets {
		var lines []string
		for _, ip := range rs.IPs {
			ipStr, ok := ip.(string)
			if !ok {
				continue
			}
			switch backend {
			case "iptables":
				lines = append(lines, fmt.Sprintf("iptables -A INPUT -p tcp -s %s --dport %d -j ACCEPT", ipStr, rs.Port))
			case "netsh":
				lines = append(lines, fmt.Sprintf("netsh advfirewall firewall add rule name=\"Allow GitHub %s port %d\" dir=in action=allow protocol=TCP remoteip=%s localport=%d", rs.Category, rs.Port, ipStr, rs.Port))
			case "ufw":
				lines = append(lines, fmt.Sprintf("ufw allow from %s to any port %d comment 'Allow GitHub %s'", ipStr, rs.Port, rs.Category))
			default:
				lines = append(lines, ipStr)
			}
		}

		outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%d.txt", rs.Category, rs.Port))
		if err := os.WriteFile(outputFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			return fmt.Errorf("failed to write to file %s: %v", outputFile, err)
		}
		color.Green("Saved %d entries to %s\n", len(lines), outputFile)
	}

	return nil
}

// printToStdout 함수는 메타 데이터를 포맷에 맞게 표준 출력으로 출력합니다.
func printToStdout(ruleSets []FirewallRuleSet, backend string) error {
	// JSON 백엔드의 경우, 보기 좋게 정렬하여 출력
	if backend == "json" {
		jsonData := make(map[string]interface{})
		for _, rs := range ruleSets {
			key := fmt.Sprintf("%s:%d", rs.Category, rs.Port)
			jsonData[key] = rs.IPs
		}
		jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %v", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	// 다른 백엔드의 경우, 카테고리 구분을 위한 주석 형태 헤더를 표준 출력으로 출력
	for _, rs := range ruleSets {
		// color 패키지를 통해 출력하면 내부적으로 stderr 등으로 라우팅될 수 있어
		// 테스트에서 stdout/stderr를 포착할 때 누락되는 문제가 있습니다.
		// 그래서 헤더는 명시적으로 fmt.Printf로 stdout에 출력합니다.
		fmt.Printf("\n#\n# Category: %s, Port: %d (%d entries)\n#\n", rs.Category, rs.Port, len(rs.IPs))

		for _, ip := range rs.IPs {
			ipStr, ok := ip.(string)
			if !ok {
				continue
			}
			switch backend {
			case "iptables":
				fmt.Printf("iptables -A INPUT -p tcp -s %s --dport %d -j ACCEPT\n", ipStr, rs.Port)
			case "netsh":
				fmt.Printf("netsh advfirewall firewall add rule name=\"Allow GitHub %s port %d\" dir=in action=allow protocol=TCP remoteip=%s localport=%d\n", rs.Category, rs.Port, ipStr, rs.Port)
			case "ufw":
				fmt.Printf("ufw allow from %s to any port %d comment 'Allow GitHub %s'\n", ipStr, rs.Port, rs.Category)
			default:
				fmt.Println(ipStr)
			}
		}
	}
	return nil
}

func main() {
	var (
		outputDir string
		types     string
		all       bool
		backend   string
		version   bool
		help      bool
		listTypes bool
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "A tool to fetch GitHub's IP addresses from the meta API.")
		fmt.Fprintln(os.Stderr, "Default behavior is to fetch all types and print them to standard output.")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintf(os.Stderr, "  # Fetch 'actions' IPs (default port 443) and 'git' IPs (default port 22) as ufw rules:\n")
		fmt.Fprintf(os.Stderr, "  %s --type actions,git --backend ufw\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Fetch 'git' IPs for both SSH and HTTPS ports and save as a structured JSON file:\n")
		fmt.Fprintf(os.Stderr, "  %s --type git:22,git:443 --backend json --output ./github_ips\n", os.Args[0])
	}

	flag.StringVar(&outputDir, "output", "", "Output directory for the generated files. If not provided, prints to standard output.")
	flag.StringVar(&types, "type", "", "Comma-separated list of types to fetch (e.g., 'actions,git:22'). If not specified, all types are fetched with their default ports.")
	flag.BoolVar(&all, "all", false, "Fetch all types with their default ports (this is the default behavior if --type is not specified).")
	flag.StringVar(&backend, "backend", "", fmt.Sprintf("Backend format for output (options: %s)", SUPPORTED_BACKENDS))
	flag.BoolVar(&version, "version", false, "Show version information.")
	flag.BoolVar(&help, "help", false, "Show this help message.")
	flag.BoolVar(&listTypes, "list-types", false, "List all available IP types with their default ports and exit.")
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	if version {
		fmt.Printf("GitHub Meta API Tool Version: %s\n", VERSION)
		return
	}

	if listTypes {
		meta, err := getMetaDatas()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching metadata: %v\n", err)
			os.Exit(1)
		}

		fmt.Fprintln(os.Stdout, "Available IP types from GitHub Meta API:")
		var availableTypes []string
		for key := range meta {
			if _, ok := meta[key].([]interface{}); ok {
				availableTypes = append(availableTypes, key)
			}
		}
		sort.Strings(availableTypes)

		for _, key := range availableTypes {
			port := getDefaultPort(key)
			fmt.Printf("- %s (default port: %d)\n", key, port)
		}
		return
	}

	meta, err := getMetaDatas()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching metadata: %v\n", err)
		os.Exit(1)
	}

	for key := range meta {
		if _, ok := meta[key].([]interface{}); ok {
			AllowedTypes = append(AllowedTypes, key)
		}
	}

	// Create a set for O(1) lookups when validating user input types
	allowedTypesSet := make(map[string]struct{}, len(AllowedTypes))
	for _, t := range AllowedTypes {
		allowedTypesSet[t] = struct{}{}
	}

	var ruleSets []FirewallRuleSet

	// 사용자가 타입을 지정하지 않으면 모든 타입을 기본 포트로 가져옵니다.
	if types == "" || all {
		for _, key := range AllowedTypes {
			if ips, ok := meta[key].([]interface{}); ok {
				port := getDefaultPort(key)
				ruleSets = append(ruleSets, FirewallRuleSet{Category: key, Port: port, IPs: ips})
			}
		}
	} else {
		typeList := strings.Split(types, ",")
		for _, t := range typeList {
			t = strings.TrimSpace(t)
			category := t
			port := 0

			if strings.Contains(t, ":") {
				parts := strings.Split(t, ":")
				category = parts[0]
				var err error
				port, err = strconv.Atoi(parts[1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: Invalid port number in '%s'\n", t)
					os.Exit(1)
				}
			} else {
				port = getDefaultPort(category)
			}

			// Validate category
			if _, ok := allowedTypesSet[category]; !ok {
				fmt.Fprintf(os.Stderr, "Error: Invalid type '%s'.\nAllowed types are: %s\n", category, strings.Join(AllowedTypes, ", "))
				os.Exit(1)
			}

			if ips, ok := meta[category].([]interface{}); ok {
				ruleSets = append(ruleSets, FirewallRuleSet{Category: category, Port: port, IPs: ips})
			}
		}
	}

	if outputDir == "" {
		if err := printToStdout(ruleSets, backend); err != nil {
			fmt.Fprintf(os.Stderr, "Error printing to stdout: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := saveToFile(ruleSets, outputDir, backend); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving files: %v\n", err)
			os.Exit(1)
		}
	}
}

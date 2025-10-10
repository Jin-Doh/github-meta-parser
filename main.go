package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

const (
	VERSION            = "0.3.0"
	GITHUB_META_URL    = "https://api.github.com/meta"
	SUPPORTED_BACKENDS = "iptables, netsh, ufw, json"
)

var (
	AllowedTypes = []string{}
)

// GetCurrentDir 함수는 명령이 실행된 곳의 위치를 반환합니다.
func getCurrentDir() string {
	_, filename, _, _ := runtime.Caller(1)
	return filepath.Dir(filename)
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

// typeCheck 함수는 사용자가 입력한 타입이 Meta 데이터의 키와 일치하는지 확인합니다.
func typeCheck(t string) bool {
	for _, at := range AllowedTypes {
		if t == at {
			return true
		}
	}
	return false
}

// saveToFile 함수는 메타 데이터를 지정된 위치에 저장합니다.
func saveToFile(meta map[string]interface{}, outputDir string, backend string) error {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		fmt.Printf("Output directory does not exist. Creating: %s\n", outputDir)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// JSON 백엔드의 경우, 모든 데이터를 하나의 파일에 저장
	if backend == "json" {
		outputFile := filepath.Join(outputDir, "github_meta.json")
		jsonData, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %v", err)
		}
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write to file %s: %v", outputFile, err)
		}
		color.Green("Saved all data to %s\n", outputFile)
		return nil
	}

	for key, value := range meta {
		ipList, ok := value.([]interface{})
		if !ok {
			continue
		}

		var lines []string
		for _, ip := range ipList {
			ipStr, ok := ip.(string)
			if !ok {
				continue
			}
			switch backend {
			case "iptables":
				lines = append(lines, fmt.Sprintf("iptables -A INPUT -p tcp -s %s -j ACCEPT", ipStr))
			case "netsh":
				lines = append(lines, fmt.Sprintf("netsh advfirewall firewall add rule name=\"Allow GitHub %s %s\" dir=in action=allow protocol=TCP remoteip=%s", key, ipStr, ipStr))
			case "ufw":
				lines = append(lines, fmt.Sprintf("ufw allow from %s to any port 22 comment 'Allow GitHub %s %s'", ipStr, key, ipStr))
			default:
				lines = append(lines, ipStr)
			}
		}

		outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.txt", key))
		if err := os.WriteFile(outputFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			return fmt.Errorf("failed to write to file %s: %v", outputFile, err)
		}
		color.Green("Saved %d entries to %s\n", len(lines), outputFile)
	}

	return nil
}

// printToStdout 함수는 메타 데이터를 포맷에 맞게 표준 출력으로 출력합니다.
func printToStdout(meta map[string]interface{}, backend string) error {
	// JSON 백엔드의 경우, 보기 좋게 정렬하여 출력
	if backend == "json" {
		jsonData, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %v", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	// 다른 백엔드의 경우, 색상있는 헤더로 카테고리 구분
	header := color.New(color.FgCyan, color.Bold)
	for key, value := range meta {
		ipList, ok := value.([]interface{})
		if !ok {
			continue
		}

		header.Printf("\n#\n# Category: %s (%d entries)\n#\n", key, len(ipList))

		for _, ip := range ipList {
			ipStr, ok := ip.(string)
			if !ok {
				continue
			}
			switch backend {
			case "iptables":
				fmt.Printf("iptables -A INPUT -p tcp -s %s -j ACCEPT\n", ipStr)
			case "netsh":
				fmt.Printf("netsh advfirewall firewall add rule name=\"Allow GitHub %s %s\" dir=in action=allow protocol=TCP remoteip=%s\n", key, ipStr, ipStr)
			case "ufw":
				fmt.Printf("ufw allow from %s to any port 22 comment 'Allow GitHub %s %s'\n", ipStr, key, ipStr)
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
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "A tool to fetch GitHub's IP addresses from the meta API.")
		fmt.Fprintln(os.Stderr, "Default behavior is to fetch all types and print them to standard output.")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintf(os.Stderr, "  # Fetch 'actions' and 'hooks' IPs and print as color-coded ufw rules:\n")
		fmt.Fprintf(os.Stderr, "  %s --type actions,hooks --backend ufw\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Fetch all IPs and save as a single structured JSON file:\n")
		fmt.Fprintf(os.Stderr, "  %s --backend json --output ./github_ips\n", os.Args[0])
	}

	flag.StringVar(&outputDir, "output", "", "Output directory for the generated files. If not provided, prints to standard output.")
	flag.StringVar(&types, "type", "", "Comma-separated list of types to fetch. If specified, overrides the default behavior of fetching all types.")
	flag.BoolVar(&all, "all", false, "Fetch all types (this is the default behavior if --type is not specified).")
	flag.StringVar(&backend, "backend", "", fmt.Sprintf("Backend format for output (options: %s)", SUPPORTED_BACKENDS))
	flag.BoolVar(&version, "version", false, "Show version information.")
	flag.BoolVar(&help, "help", false, "Show this help message.")
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	if version {
		fmt.Printf("GitHub Meta API Tool Version: %s\n", VERSION)
		return
	}

	meta, err := getMetaDatas()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching metadata: %v\n", err)
		os.Exit(1)
	}

	for key := range meta {
		AllowedTypes = append(AllowedTypes, key)
	}

	fetchAll := true
	if types != "" {
		fetchAll = false
	} else if all {
		fetchAll = true
	}

	var selectedMeta map[string]interface{}
	if fetchAll {
		selectedMeta = meta
	} else {
		selectedMeta = make(map[string]interface{})
		typeList := strings.Split(types, ",")
		for _, t := range typeList {
			t = strings.TrimSpace(t)
			if !typeCheck(t) {
				fmt.Fprintf(os.Stderr, "Error: Invalid type '%s'.\nAllowed types are: %s\n", t, strings.Join(AllowedTypes, ", "))
				os.Exit(1)
			}
			if value, exists := meta[t]; exists {
				selectedMeta[t] = value
			}
		}
	}

	if outputDir == "" {
		if err := printToStdout(selectedMeta, backend); err != nil {
			fmt.Fprintf(os.Stderr, "Error printing to stdout: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := saveToFile(selectedMeta, outputDir, backend); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving files: %v\n", err)
			os.Exit(1)
		}
	}
}

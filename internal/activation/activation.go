package activation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// Display shows the activation code prominently in the console with a QR code
// pointing to the dashboard where the user authenticates and enters the code.
func Display(code string, dashboardURL string) {
	activateURL := fmt.Sprintf("%s/activate?code=%s", dashboardURL, code)

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════╗")
	fmt.Println("  ║                                                          ║")
	fmt.Println("  ║   🌶  HABANERO AGENT — ACTIVATION REQUIRED              ║")
	fmt.Println("  ║                                                          ║")
	fmt.Printf("  ║   Code:  %s                    ║\n", code)
	fmt.Println("  ║                                                          ║")
	fmt.Println("  ║   Scan the QR code or visit:                             ║")
	fmt.Printf("  ║   %s\n", padRight(activateURL, 56)+"║")
	fmt.Println("  ║                                                          ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Generate QR code in terminal using Unicode block characters
	printQR(activateURL)

	fmt.Println()
	fmt.Printf("  1. Open the URL above (or scan the QR code)\n")
	fmt.Printf("  2. Sign in with Google, Apple, or your SSO provider\n")
	fmt.Printf("  3. Enter code: %s\n", code)
	fmt.Printf("  4. This agent will automatically connect to your fleet\n")
	fmt.Println()
	fmt.Println("  Waiting for activation...")
}

// WaitForActivation polls the API until the code is consumed, then returns the config
func WaitForActivation(code string, apiEndpoint string, logger *slog.Logger) (*ActivationResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/fleet/activation/status?code=%s", apiEndpoint, code), nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			logger.Debug("activation poll failed, retrying", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		var result struct {
			OK             bool   `json:"ok"`
			Status         string `json:"status"` // "pending", "activated", "expired"
			ConsultantKey  string `json:"consultant_key,omitempty"`
			ConsultantName string `json:"consultant_name,omitempty"`
			SiteName       string `json:"site_name,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		resp.Body.Close()

		switch result.Status {
		case "activated":
			logger.Info("activation successful", "consultant", result.ConsultantName, "site", result.SiteName)
			return &ActivationResult{
				ConsultantKey:  result.ConsultantKey,
				ConsultantName: result.ConsultantName,
				SiteName:       result.SiteName,
			}, nil
		case "expired":
			return nil, fmt.Errorf("activation code expired")
		default:
			// Still pending, wait and retry
			time.Sleep(5 * time.Second)
		}
	}
}

type ActivationResult struct {
	ConsultantKey  string
	ConsultantName string
	SiteName       string
}

// GenerateCode requests a new activation code from the API
func GenerateCode(apiEndpoint string, hostname string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	body := fmt.Sprintf(`{"hostname":"%s","device_id":"%s"}`, hostname, getDeviceID())
	resp, err := client.Post(
		apiEndpoint+"/fleet/activation/request",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK   bool   `json:"ok"`
		Code string `json:"activation_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK || result.Code == "" {
		return "", fmt.Errorf("failed to generate activation code")
	}
	return result.Code, nil
}

func getDeviceID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("agent-%s-%d", hostname, time.Now().Unix()%100000)
}

// printQR generates a QR code in the terminal using Unicode block characters
func printQR(url string) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		// Fallback: just show the URL
		fmt.Printf("  %s\n", url)
		return
	}

	bitmap := qr.Bitmap()
	for y := 0; y < len(bitmap)-1; y += 2 {
		fmt.Print("  ")
		for x := 0; x < len(bitmap[y]); x++ {
			top := bitmap[y][x]
			bot := false
			if y+1 < len(bitmap) {
				bot = bitmap[y+1][x]
			}
			// Use Unicode half-blocks to fit 2 rows per line
			switch {
			case top && bot:
				fmt.Print("█")
			case top && !bot:
				fmt.Print("▀")
			case !top && bot:
				fmt.Print("▄")
			default:
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}

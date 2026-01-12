package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// IPResponse ответ с IP адресом клиента
type IPResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

type IPInfo struct {
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

func getCountryByIP(ip string) (string, error) {
	// метод из сервиса, котрый по ip возвращает страну
	// Лишь 45 запросов в минуту для бесплатной версии
	url := "http://ip-api.com/json/" + ip

	resp, err := http.Get(url)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info IPInfo

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}

	return info.Country, nil
}

// getClientIP извлекает реальный IP адрес клиента из запроса
func getClientIP(r *http.Request) string {
	// Проверяем заголовки прокси (X-Forwarded-For, X-Real-IP)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For может содержать несколько IP через запятую
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		if net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	// Если заголовков нет, используем RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Если нет порта, пробуем весь RemoteAddr
		if net.ParseIP(r.RemoteAddr) != nil {
			return r.RemoteAddr
		}
		return ""
	}

	return host
}

func shouldUseServerIP(clientIP string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// Проверяем WireGuard диапазон (10.7.0.0/24)
	if strings.HasPrefix(clientIP, "10.7.") {
		return true
	}

	// Проверяем общие приватные сети
	return ip.IsPrivate() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast()
}

// ipHandler возвращает публичный IP клиента
func ipHandler(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)

	if clientIP == "" {
		http.Error(w, "Не удалось определить IP", http.StatusInternalServerError)
		return
	}

	var displayIP string
	if shouldUseServerIP(clientIP) {
		displayIP = serverPublicIP
	} else {
		displayIP = clientIP
	}

	country, err := getCountryByIP(displayIP)

	if err != nil {
		http.Error(w, "Не удалось определить страну", http.StatusInternalServerError)
		return
	}

	response := IPResponse{
		IP:      displayIP,
		Country: country,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

func getServerPublicIP() (string, error) {
	// Один раз при старте получаем IP сервера
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

var serverPublicIP string

func main() {

	ip, err := getServerPublicIP()
	if err == nil {
		serverPublicIP = ip
	}

	port := ":8080"

	http.HandleFunc("/ip", ipHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			ipHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	log.Printf("Сервер запущен на порту %s", port)
	log.Printf("Endpoint: http://0.0.0.0%s/ip", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

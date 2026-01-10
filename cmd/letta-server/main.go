package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
)

// IPResponse ответ с IP адресом клиента
type IPResponse struct {
	IP string `json:"ip"`
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

// ipHandler возвращает публичный IP клиента
func ipHandler(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)

	if clientIP == "" {
		http.Error(w, "Не удалось определить IP", http.StatusInternalServerError)
		return
	}

	response := IPResponse{
		IP: clientIP,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

func main() {
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

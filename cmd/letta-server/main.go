package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client
var ctx = context.Background()

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // или твой Redis сервер
		Password: "",               // пароль если есть
		DB:       0,                // номер базы
	})

	// Проверяем подключение
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("⚠️  Redis недоступен: %v (работаем без кэша)", err)
		redisClient = nil
	} else {
		log.Println("✅ Redis подключен")
	}
}

type IPInfo struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

func getCountryByIPWithCache(ip string) (string, string, error) {
	if redisClient == nil {
		return getCountryByIPDirect(ip)
	}

	// Пробуем получить как JSON
	cached, err := redisClient.Get(ctx, "ip:"+ip).Result()
	if err == nil && cached != "" {
		var data struct {
			Country string `json:"country"`
			Code    string `json:"code"`
		}
		if json.Unmarshal([]byte(cached), &data) == nil {
			return data.Country, data.Code, nil
		}
	}

	// Если нет - идем к API
	log.Printf("Не удалось найти %s в кеше, идем в сервис http://ip-api.com/json/", ip)
	country, code, err := getCountryByIPDirect(ip)
	if err != nil {
		return "", "", err
	}

	// Сохраняем как JSON
	data := map[string]string{
		"country": country,
		"code":    code,
	}
	jsonData, _ := json.Marshal(data)
	redisClient.Set(ctx, "ip:"+ip, jsonData, 24*time.Hour)

	return country, code, nil
}

// Старая функция переименовываем
// Изменяем только getCountryByIPDirect
func getCountryByIPDirect(ip string) (string, string, error) { // Возвращаем 2 строки
	url := "http://ip-api.com/json/" + ip

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result struct {
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		Status      string `json:"status"`
		Message     string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if result.Status != "success" {
		return "", "", fmt.Errorf("ip-api error: %s", result.Message)
	}

	return result.Country, result.CountryCode, nil
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

	// Получаем ОБЕ информации
	country, countryCode, err := getCountryByIPWithCache(displayIP)
	if err != nil {
		log.Printf("⚠️  Ошибка получения страны для %s: %v", displayIP, err)
		country = ""
		countryCode = ""
	}

	response := IPInfo{
		IP:          displayIP,
		Country:     country,
		CountryCode: countryCode,
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
	log.Println("🚀 Запускаем letta-server...")

	initRedis()

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

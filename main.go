package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type subsPayload struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Lists  [1]int `json:"lists"`
}

type Config struct {
	BaseUrl         string `env:"BASE_URL"`
	MailServerToken string `env:"MAIL_SERVER_TOKEN"`
	MailServerUrl   string `env:"MAIL_SERVER_URL"`
	NewsUrl         string `env:"NEWS_URL"`
}

var cfg Config

func validateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func sendWelcomeEmail(email string) error {

	payload := strings.NewReader(
		fmt.Sprintf(
			"{\n  \"subscriber_email\": \"%s\",\n  \"template_id\": 3,\n  \"content_type\": \"html\"\n}",
			email,
		),
	)

	request, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/tx", cfg.MailServerUrl),
		payload,
	)

	if err != nil {
		return fmt.Errorf("impossible to build request: %w", err)
	}

	request.Header.Set("authorization", cfg.MailServerToken)
	request.Header.Set("content-type", "application/json")

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("impossible to send request: %w", err)
	}
	log.Printf("status Code: %d", res.StatusCode)

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("impossible to read all body of response: %w", err)
	}
	log.Printf("res body: %s", string(resBody))

	return nil
}

func subscribe(w http.ResponseWriter, req *http.Request) {

	err := req.ParseForm()
	if err != nil {
		http.Error(w, "Invalid Data", http.StatusBadRequest)
		return
	}

	email := req.Form["email"]
	fname := req.Form["firstname"]
	lname := req.Form["lastname"]

	// Check if required fields are present
	if len(email) == 0 || email[0] == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	if len(fname) == 0 || fname[0] == "" {
		http.Error(w, "First name is required", http.StatusBadRequest)
		return
	}
	if len(lname) == 0 || lname[0] == "" {
		http.Error(w, "Last name is required", http.StatusBadRequest)
		return
	}

	if !(validateEmail(email[0])) {
		http.Error(w, "Invalid Email", http.StatusBadRequest)
		return
	}
	payload := &subsPayload{
		Email:  email[0],
		Name:   fmt.Sprintf("%s %s", fname[0], lname[0]),
		Status: "enabled",
		Lists:  [1]int{1},
	}

	marshal, err := json.Marshal(payload)

	// Check if parsing json failed
	if err != nil {
		log.Fatalf("Impossible to marshal payload: %s", err)
		http.Error(w, "Subscription failed", http.StatusInternalServerError)
		return
	}

	// send request to listmonk for adding subscription

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/api/subscribers", cfg.MailServerUrl), bytes.NewReader(marshal))

	if err != nil {
		log.Fatalf("Impossible to build request: %s", err)
		http.Error(w, "Subscription failed", http.StatusInternalServerError)
		return
	}

	request.Header.Set("authorization", cfg.MailServerToken)
	request.Header.Set("content-type", "application/json")

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(request)
	if err != nil {
		log.Fatalf("impossible to send request: %s", err)
	}
	log.Printf("status Code: %d", res.StatusCode)

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("impossible to read all body of response: %s", err)
	}
	log.Printf("res body: %s", string(resBody))

	err = sendWelcomeEmail(email[0])
	if err != nil {
		log.Printf("Failed to send welcome email: %v", err)
	}
}

func blog(w http.ResponseWriter, req *http.Request) {

	theme := req.URL.Query().Get("theme")
	data, err := getPageList()
	if err != nil {
		fmt.Printf("blog(): failed to build list of page entries %s", err)
	}
	out := renderMarkdown(data, 120, theme)

	if _, err := w.Write([]byte(out)); err != nil {
		fmt.Printf("Failed to return response for path /blog %s", err)
		http.Error(w, "Server Failed to process the request", http.StatusInternalServerError)
	}
}

func news(w http.ResponseWriter, req *http.Request) {

	theme := req.URL.Query().Get("theme")
	result, err := getHtml(req.PathValue("id"), theme)
	if err != nil {
		switch {
		case errors.Is(err, NegativeId):
			fmt.Printf("Bad request recieved for a blog post with negative index %s", err)
			http.Error(w, "Blog pages can not have negative values", http.StatusBadRequest)
		default:
			fmt.Printf("Failes to process the request for path / %s", err)
			http.Error(w, "Server Failed to process the request", http.StatusInternalServerError)
		}
	}
	if _, err := w.Write([]byte(result)); err != nil {
		fmt.Printf("Failed to return response for path / %s", err)
		http.Error(w, "Server Failed to process the request", http.StatusInternalServerError)
	}
}

func help(w http.ResponseWriter, req *http.Request) {
	theme := req.URL.Query().Get("theme")
	dat, err := os.ReadFile("help.md")
	if err != nil {
		fmt.Printf("Error occured while reading the file %s", err)
		http.Error(w, "Server Failed to process the request", http.StatusInternalServerError)
	}
	out := renderMarkdown(string(dat), 120, theme)
	if _, err := w.Write([]byte(out)); err != nil {
		fmt.Printf("Failed to return response for path / %s", err)
		http.Error(w, "Server Failed to process the request", http.StatusInternalServerError)
	}
}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		panic(err)
	}

	err = env.Parse(&cfg)
	if err != nil {
		panic(err)
	}

	getXML()
	http.HandleFunc("/", news)
	http.HandleFunc("/{id}", news)
	http.HandleFunc("/blog", blog)
	http.HandleFunc("/help", help)
	http.HandleFunc("/subscribe", subscribe)
	http.ListenAndServe(":8090", nil)
}

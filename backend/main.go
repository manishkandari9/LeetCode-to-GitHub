package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	githubClientID     string
	githubClientSecret string
	redirectURI        string
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ Warning: Could not load .env file")
	}

	githubClientID = getEnv("GITHUB_CLIENT_ID")
	githubClientSecret = getEnv("GITHUB_CLIENT_SECRET")
	redirectURI = "https://emppmgemkbjiojiblefmidpoichmbggg.chromiumapp.org" // Hardcoded to match Chrome extension
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("❌ Missing environment variable: %s", key)
	}
	return value
}

func getGitHubLoginURL() string {
	return fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=repo", githubClientID, redirectURI)
}

func getGitHubAccessToken(code string) (string, error) {
	url := "https://github.com/login/oauth/access_token"
	requestBody, _ := json.Marshal(map[string]string{
		"client_id":     githubClientID,
		"client_secret": githubClientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("❌ Request creation failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("❌ Failed to fetch access token: %v", err)
	}
	defer resp.Body.Close()

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("❌ Failed to parse response: %v", err)
	}

	accessToken, exists := response["access_token"]
	if !exists {
		return "", fmt.Errorf("❌ No access token found in response")
	}

	return accessToken, nil
}

func pushToGitHub(accessToken, repo, fileName, content string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, fileName)
	encodedContent := base64.StdEncoding.EncodeToString([]byte(content))

	client := &http.Client{Timeout: 10 * time.Second}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Failed to check file existence: %v", err)
	}
	defer resp.Body.Close()

	var sha string
	if resp.StatusCode == http.StatusOK {
		var fileData map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&fileData)
		sha, _ = fileData["sha"].(string)
	}

	fileData := map[string]string{
		"message": "🚀 Auto-update: " + fileName,
		"content": encodedContent,
	}
	if sha != "" {
		fileData["sha"] = sha
	}
	jsonData, _ := json.Marshal(fileData)

	req, err = http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("❌ Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ GitHub API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ GitHub API error: %s", resp.Status)
	}

	log.Println("✅ Successfully pushed to GitHub:", fileName)
	return nil
}

func main() {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.GET("/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"url": getGitHubLoginURL()})
	})

	r.GET("/emppmgemkbjiojiblefmidpoichmbggg.chromiumapp.org", func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Missing authorization code"})
			return
		}
		accessToken, err := getGitHubAccessToken(code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "access_token": accessToken})
	})

	r.POST("/save", func(c *gin.Context) {
		var req struct {
			AccessToken string `json:"access_token"`
			Filename    string `json:"filename"`
			Content     string `json:"content"`
			Repo        string `json:"repo"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Invalid request"})
			return
		}
		if req.Repo == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "❌ Repository is required"})
			return
		}
		err := pushToGitHub(req.AccessToken, req.Repo, req.Filename, req.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "✅ Solution saved to GitHub"})
	})

	log.Println("🚀 Server running on port 8080...")
	r.Run(":8080")
}
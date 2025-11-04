package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type TurnHandler struct{}

func NewTurnHandler() *TurnHandler {
	return &TurnHandler{}
}

type CloudflareTurnRequest struct {
	TTL int `json:"ttl"`
}

type CloudflareTurnResponse struct {
	IceServers []struct {
		URLs       []string `json:"urls"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
	} `json:"iceServers"`
}

func (h *TurnHandler) GetTurnCredentials(c *gin.Context) {
	turnTokenID := os.Getenv("CLOUDFLARE_TURN_TOKEN_ID")
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")

	url := fmt.Sprintf("https://rtc.live.cloudflare.com/v1/turn/keys/%s/credentials/generate-ice-servers", turnTokenID)
	
	reqBody := CloudflareTurnRequest{
		TTL: 86400,
	}
	
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to call Cloudflare API: %v", err)})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read response: %v", err)})
		return
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Cloudflare API returned status %d: %s", resp.StatusCode, string(body)),
		})
		return
	}

	var cfResponse CloudflareTurnResponse
	if err := json.Unmarshal(body, &cfResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to parse Cloudflare response: %v. Body: %s", err, string(body)),
		})
		return
	}

	if len(cfResponse.IceServers) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Cloudflare API returned empty iceServers. Response: %s", string(body)),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"iceServers": cfResponse.IceServers,
	})
}


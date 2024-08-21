package gemini

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/google/generative-ai-go/genai"
	"github.com/rendizi/stay-connected-inst/config"
	"github.com/rendizi/stay-connected-inst/pkg/logger"
	"google.golang.org/api/option"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Helper function to generate a random string
func generateRandomString(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz" // Lowercase only and no dashes
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	for i := 0; i < length; i++ {
		bytes[i] = chars[int(bytes[i])%len(chars)]
	}
	logger.Info(string(bytes))
	return string(bytes), nil
}

func getFileURIFromURL(fileURL string) (string, error) {
	// Ensure the URL is valid
	if !strings.HasPrefix(fileURL, "http://") && !strings.HasPrefix(fileURL, "https://") {
		return "", fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	// Simulate the process of getting a URI. In this case, we assume the URL itself is the URI.
	// If additional processing is required by the Gemini API, you would add it here.
	return fileURL, nil
}

func downloadFile(url string) (io.Reader, string, error) {
	// Generate a random name for the file
	randomName, err := generateRandomString(30)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate random name: %w", err)
	}

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	// Return the response body as io.Reader
	return resp.Body, randomName, nil
}

func SummarizeVideo(fileURL string, promptText string) (string, int, bool, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.Config.GeminiKey))
	if err != nil {
		return "", 0, false, err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash")

	// Download the file from the URL
	reader, fileName, err := downloadFile(fileURL)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		if closer, ok := reader.(io.Closer); ok {
			closer.Close()
		}
	}()

	// Sanitize the file name
	sanitizedFileName := strings.ToLower(fileName)
	sanitizedFileName = strings.Trim(sanitizedFileName, "-")
	logger.Info(sanitizedFileName)

	// Upload the file
	uploadedFile, err := client.UploadFile(ctx, sanitizedFileName, reader, &genai.UploadFileOptions{
		MIMEType: "video/mp4",
	})
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to upload file: %w", err)
	}

	// Check the file processing state
	for uploadedFile.State == genai.FileStateProcessing {
		time.Sleep(5 * time.Second)
		uploadedFile, err = client.GetFile(ctx, uploadedFile.Name)
		if err != nil {
			return "", 0, false, fmt.Errorf("failed to get file status: %w", err)
		}
	}

	if uploadedFile.State != genai.FileStateActive {
		return "", 0, false, fmt.Errorf("uploaded file has state %s, not active", uploadedFile.State)
	}

	// Create the prompt
	prompt := []genai.Part{
		genai.FileData{
			URI: uploadedFile.URI,
		},
		genai.Text(promptText),
	}

	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to generate content: %w", err)
	}

	// Collect summary content
	for _, c := range resp.Candidates {
		if c.Content != nil {
			var data map[string]interface{}
			err = json.Unmarshal([]byte(fmt.Sprintf("%s\n", c.Content.Parts[0])), &data)
			if err != nil {
				return "", 0, false, err
			}
			description := data["description"].(string)
			addIt := data["addIt"].(bool)
			length := data["clip_length"].(float64)
			log.Println(data)

			return description, int(length), addIt, nil
		}
	}

	return "", 0, false, nil
}

package shotstack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type Data struct {
	Timeline Timeline `json:"timeline"`
	Output   struct {
		Format string `json:"format"`
		Size   struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"size"`
	} `json:"output"`
}

type Timeline struct {
	Soundtrack struct {
		Src    string `json:"src"`
		Effect string `json:"effect"`
	} `json:"soundtrack"`
	Tracks []Track `json:"tracks"`
}

type Asset struct {
	Type   string `json:"type"`
	Src    string `json:"src"`
	Length int
}

type Transition struct {
	In  string `json:"in,omitempty"`
	Out string `json:"out"`
}

type Clip struct {
	Asset      Asset      `json:"asset"`
	Start      int        `json:"start"`
	Length     int        `json:"length"`
	Effect     string     `json:"effect"`
	Transition Transition `json:"transition"`
}

func GenerateVideoJson(medias []Asset) (Data, error) {
	rand.Seed(time.Now().UnixNano())

	effects := []string{"zoomIn", "slideUp", "slideLeft", "zoomOut", "slideDown", "slideRight"}

	var clips []Clip
	start := 0

	for i, media := range medias {
		var clip Clip
		clip.Asset = Asset{Type: media.Type, Src: media.Src}
		clip.Start = start

		if media.Length == 0 {
			continue
		}

		clip.Length = media.Length

		clip.Effect = effects[rand.Intn(len(effects))]

		if i == 0 {
			clip.Transition = Transition{
				In:  "fade",
				Out: "fade",
			}
		} else {
			clip.Transition = Transition{
				Out: "fade",
			}
		}

		clips = append(clips, clip)

		start += clip.Length - 1
	}

	var resultRequest Data
	resultRequest.Output.Format = "mp4"
	resultRequest.Output.Size.Width = 720
	resultRequest.Output.Size.Height = 1280
	var timeline Timeline
	timeline.Tracks = []Track{
		{Clips: clips},
	}
	timeline.Soundtrack.Src = "https://shotstack-assets.s3-ap-southeast-2.amazonaws.com/music/freepd/advertising.mp3"
	timeline.Soundtrack.Effect = "fadeInFadeOut"
	resultRequest.Timeline = timeline
	log.Println(resultRequest)
	return resultRequest, nil
}

type Track struct {
	Clips []Clip `json:"clips"`
}

func GenerateVideo(request Data) (string, error) {
	requestJson, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	apiKey := os.Getenv("SHOTSTACK_API_KEY")
	if apiKey == "" {
		return "", errors.New("SHOTSTACK_API_KEY not set in environment")
	}

	req, err := http.NewRequest("POST", "https://api.shotstack.io/edit/stage/render", bytes.NewBuffer(requestJson))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if success, ok := result["success"].(bool); !ok || !success {
		log.Println(success)
		return "", errors.New("failed to queue render: " + result["message"].(string))
	}

	responseData, ok := result["response"].(map[string]interface{})
	if !ok {
		return "", errors.New("invalid response format")
	}

	renderID, ok := responseData["id"].(string)
	if !ok {
		log.Println(responseData)
		return "", errors.New("render ID not found or not a string")
	}

	return renderID, nil

	return renderID, nil
}

func GetUrl(id string) (string, error) {
	for {
		url := fmt.Sprintf("https://api.shotstack.io/edit/stage/render/%s", id)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatal("Error creating request:", err)
		}

		req.Header.Set("x-api-key", os.Getenv("SHOTSTACK_API_KEY"))

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Error sending request:", err)
		}

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			log.Fatal("Error decoding response body:", err)
		}

		log.Println(response)

		success, ok := response["success"].(bool)
		if !ok {
			log.Fatal("Invalid response format, 'success' field not found or not a boolean")
		}

		if success {
			url, ok = response["response"].(map[string]interface{})["url"].(string)
			if !ok {
				time.Sleep(25 * time.Second)
			} else {
				resp.Body.Close()
				return url, nil
			}

		} else {
			fmt.Println("Render was not successful. Retrying in 5 seconds...")
			time.Sleep(25 * time.Second)
		}

		resp.Body.Close()
	}
}

package openai

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rendizi/stay-connected-inst/pkg/logger"
	"os"
)

func SummarizeImage(url string, prompt string) (string, bool, error) {
	apiEndpoint := "https://api.openai.com/v1/chat/completions"

	apiKey := os.Getenv("OPENAI_KEY")
	client := resty.New()

	response, err := client.R().
		SetAuthToken(apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"model": "gpt-4o",
			"response_format": map[string]interface{}{
				"type": "json_object",
			},
			"messages": []interface{}{
				map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": prompt,
						},
						map[string]interface{}{
							"type": "image_url",
							"image_url": map[string]interface{}{
								"url": url,
							},
						},
					},
				},
			},
			"max_tokens": 75,
		}).
		Post(apiEndpoint)

	if err != nil {
		return "", false, fmt.Errorf("Error while sending the request: %v", err)
	}

	body := response.Body()

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", false, fmt.Errorf("Error while decoding JSON response: %v", err)
	}

	choices := data["choices"].([]interface{})
	if len(choices) == 0 {
		return "", false, fmt.Errorf("No choices found in the response")
	}

	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(message), &result)
	if err != nil {
		return "", false, fmt.Errorf("Error while decoding message content JSON: %v", err)
	}

	description, ok := result["description"].(string)
	if !ok {
		return "", false, fmt.Errorf("No description found in the response")
	}

	addIt, ok := result["addIt"].(bool)
	if !ok {
		addIt = false
	}

	return description, addIt, nil
}

type StoriesType struct {
	Author    string
	Summarize string
}

func SummarizeImagesToOne(userPrompt []StoriesType, busines bool, preferences string) (string, error) {
	apiKey := os.Getenv("OPENAI_KEY")
	apiEndpoint := "https://api.openai.com/v1/chat/completions"

	client := resty.New()
	content := "You are given array of storieses summarize. I am very busy so give the most interesting ones, make them shorter without losing an idea. Maximum symbols-100, don't use markup symbols. Response should be like 1 text, no need to divide into ordered/unordered list. If is is empty or there is information not interesting and not related with someone's life- return 'Nothing interesting'. Also write how can I start dialog with him, suggest some action . Write simple. User's preferences: " + preferences
	if busines {
		content = "You are given array of storieses summarize of some busines account. I am very buse so give the most interesting ones, make them shorter without losing an idea. Maximum symbols-100, dont use markup symbols. Response should be like 1 text, no need to divide into ordered/unordered list. If it is epty or there is no interestings inferomation, news or info that can be helpful for concurents - return 'Nothing interesting'. Wrtie simple. User's preferences: " + preferences
	}
	logger.Info(content)

	response, err := client.R().
		SetAuthToken(apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"model": "gpt-4o",
			"messages": []interface{}{
				map[string]interface{}{
					"role":    "system",
					"content": content},
				map[string]interface{}{
					"role":    "user",
					"content": fmt.Sprintf("%s", userPrompt),
				},
			},
			"max_tokens": 100,
		}).
		Post(apiEndpoint)

	if err != nil {
		return "", fmt.Errorf("Error while sending send the request: %v", err)
	}

	body := response.Body()

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", fmt.Errorf("Error while decoding JSON response:", err)
	}

	content = data["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
	return content, nil
}

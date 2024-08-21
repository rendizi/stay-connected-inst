package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/rendizi/stay-connected-inst/config"
	"github.com/rendizi/stay-connected-inst/internal/grpc"
	inst2 "github.com/rendizi/stay-connected-inst/internal/inst"
	"github.com/rendizi/stay-connected-inst/internal/redis"
	"github.com/rendizi/stay-connected-inst/internal/services/gemini"
	"github.com/rendizi/stay-connected-inst/internal/services/openai"
	"github.com/rendizi/stay-connected-inst/internal/services/shotstack"
	"github.com/rendizi/stay-connected-inst/pkg/logger"
	"go.uber.org/zap"
	"log"
	"time"
)

type Server struct {
	grpc.UnimplementedStoriesSummarizerServer
}

func (s *Server) QueueLength(ctx context.Context, req *grpc.QueueLengthRequest) (*grpc.QueueLengthResponse, error) {
	length := config.GetQueueLength()
	return &grpc.QueueLengthResponse{Response: float32(length)}, nil
}

func (s *Server) SummarizeStories(req *grpc.SummarizeStoriesRequest, stream grpc.StoriesSummarizer_SummarizeStoriesServer) error {
	isDaily := req.IsDaily
	usernames := req.Usernames
	logger.Info(fmt.Sprintf("%v", usernames))
	left := req.GetLeft()
	preferences := req.UserPreferences
	if len(preferences) == 0 {
		return nil
	}
	if left <= 0 {
		if err := stream.Send(Format("You have reacher your usage limit")); err != nil {
			return err
		}
		return nil
	}
	if len(usernames) <= 0 {
		if err := stream.Send(Format("No usernames has been provided")); err != nil {
			return err
		}
		return nil
	}
	id := fmt.Sprintf("%s", uuid.New())
	config.Enqueue(id, len(usernames))
	defer config.RemoveFromQueue(id, len(usernames))
	if err := stream.Send(Format("Queued")); err != nil {
		return err
	}
	signal := make(chan struct{})
	storiesArray := make([]openai.StoriesType, 0)

	go func() {
		defer close(signal)
		for {
			if config.NextInQueue() == id {
				signal <- struct{}{}
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()
	<-signal

	inst, err := inst2.Login(config.Config.DefaultLogin, config.Config.DefaultPassword)
	if err != nil {
		if err2 := stream.Send(Format(fmt.Sprintf("Failed to login to instagram: %s", err.Error()))); err2 != nil {
			return err2
		}
		logger.Error("Failed to login to Instagram", zap.Error(err))
		return err
	}
	if err = stream.Send(Format("Logged in to instagram")); err != nil {
		return err
	}
	var data string
	medias := make([]shotstack.Asset, 0)
	var used float32
	used = 0

	for _, username := range usernames {

		data, _, err = redis.GetSummarizes(context.Background(), username)
		if err != nil {
			logger.Error("Error retrieving summarizes from Redis", zap.String("username", username), zap.Error(err))
		}
		if err = stream.Send(Format("")); err != nil {
			return err
		}

		// Unmarshal data to []string
		var thisWeek []string
		err = json.Unmarshal([]byte(data), &thisWeek)
		if err != nil {
			logger.Error("Error unmarshalling data for user", zap.String("username", username), zap.Error(err))
		}
		logger.Info("Unmarshalled this week's data for user", zap.String("username", username), zap.Strings("thisWeek", thisWeek))

		// Visit profile
		if err = stream.Send(Format("Visiting profile")); err != nil {
			return err
		}
		profile, err := inst.VisitProfile(username)
		if err != nil {
			logger.Error("Error visiting profile", zap.String("username", username), zap.Error(err))
			continue
		}
		logger.Info("Visited profile for user", zap.String("username", username), zap.String("profile", profile.User.Username))

		// Getting stories
		if err = stream.Send(Format("Getting stories")); err != nil {
			return err
		}
		storiess, err := profile.User.Stories()
		if err != nil {
			logger.Error("Error fetching stories", zap.String("username", username), zap.Error(err))
			continue
		}
		logger.Info("Fetched stories", zap.String("username", username), zap.Any("stories", storiess))

		temp := make([]openai.StoriesType, 0)
		usedIsMoreThanLeft := false

		for _, story := range storiess.Reel.Items {
			if usedIsMoreThanLeft {
				break
			}
			var prompt string
			var addIt bool
			var resp string
			var val string
			var clip_length int
			var was_video bool
			for _, media := range story.Videos {
				was_video = true
				if usedIsMoreThanLeft {
					break
				}
				val, addIt, err = redis.GetSummarizes(context.Background(), media.URL)
				if err != nil {
					if !profile.User.IsBusiness {
						prompt = fmt.Sprintf("I have a video from an %s's(use it when want to write about him instead of writing 'user') Instagram story. Your task is to determine if it contains any interesting or relevant information about the person's life or news. If it does, summarize this information in 1 short sentence. If the video content is not related to the person's personal life, not interesting or important activities/news, return following response: 'Nothing interesting'. Give logically connected summarize based on previous storieses(if it is empty- don't say me it is empty, give result only based on video or return empty response):%s.Last 7 days stories: %s. Don't repeat what is already summarized and in old storieses. Additional stories info: events: %s, hashtags: %s, polls: %s, locations: %s, questions: %s, sliders: %s, mentions: %s. Maximum tokens: 75, write it as simple as possible, like people would say, use simple words. Response should be in following json format: {\"description\":string,\"addIt\":bool,\"clip_length\":int}. If you think that this stories should be added to short recap video- addIt true, otherwise false. If addIt is true say what's length in seconds it should be in clip as \"clip_length\"",
							story.User.Username, temp, data, story.StoryEvents, story.StoryHashtags, story.StoryPolls, story.StoryLocations, story.StorySliders, story.StoryQuestions, story.Mentions)
					} else {
						prompt = fmt.Sprintf("I have a video from an %s's(use it when want to write about him instead of writing 'user') Instagram story. Your task is to determine if it contains any interesting or relevant information about the busines's news or sales. If it does, summarize this information in 1 short sentence. If the video content is not interesting or important activities/news, return following response: 'Nothing interesting'. Give logically connected summarize based on previous storieses(if it is empty- don't say me it is empty, give result only based on video or return empty response):%s.Last 7 days stories: %s. Don't repeat what is already summarized and in old storieses. Additional stories info: events: %s, hashtags: %s, polls: %s, locations: %s, questions: %s, sliders: %s, mentions: %s. Maximum tokens: 75, write it as simple as possible, like people would say, use simple words. Response should be in following json format: {\"description\":string,\"addIt\":bool,\"clip_length\":int}. If you think that this stories should be added to short recap video- addIt true, otherwise false . If addIt is true say what's length in seconds it should be in clip as \"clip_length\"",
							story.User.Username, temp, data, story.StoryEvents, story.StoryHashtags, story.StoryPolls, story.StoryLocations, story.StorySliders, story.StoryQuestions, story.Mentions)
					}

					resp, clip_length, addIt, err = gemini.SummarizeVideo(media.URL, prompt)
					used += 1
					if used >= left {
						usedIsMoreThanLeft = true
						break
					}
					if err != nil {
						logger.Error("Error summarizing video", zap.String("URL", media.URL), zap.Error(err))
						continue
					}
					if addIt {
						var tempAsset shotstack.Asset
						tempAsset.Type = "video"
						tempAsset.Src = media.URL
						tempAsset.Length = clip_length
						medias = append(medias, tempAsset)
					}
					err = redis.StoreSummarizes(context.Background(), media.URL, map[string]interface{}{"value": resp, "addIt": addIt}, "", 24*time.Hour)
					if err != nil {
						logger.Error("Error storing summarized video in Redis", zap.String("URL", media.URL), zap.Error(err))
						continue
					}
				} else {
					resp = val
				}
				if resp != "Nothing interesting" || resp != "Nothing interesting." {
					var tempStoriesType openai.StoriesType
					tempStoriesType.Author = story.User.Username
					tempStoriesType.Summarize = resp
					if profile.User.Friendship.FollowedBy {
						temp = append([]openai.StoriesType{tempStoriesType}, temp...)
					} else {
						temp = append(temp, tempStoriesType)
					}
				}
				if err = stream.Send(Format(resp)); err != nil {
					return err
				}
				break
			}
			if was_video {
				continue
			}
			for _, media := range story.Images.Versions {
				val, addIt, err = redis.GetSummarizes(context.Background(), media.URL)
				if err != nil {
					if !profile.User.IsBusiness {
						prompt = fmt.Sprintf("I have an image from an %s's(use it when want to write about him instead of writing 'user') Instagram story. Your task is to determine if it contains any interesting or relevant information about the person's life or news. If it does, summarize this information in 1 short sentence. If the image content is not related to the person's personal life, not interesting or important activities/news, return following response: 'Nothing interesting'. Give logically connected summarize based on previous storieses(if it is empty- don't say me it is empty, give result only based on photo or return empty response):%s.Last 7 days stories: %s. Don't repeat what is already summarized and in old storieses. Additional stories info: events: %s, hashtags: %s, polls: %s, locations: %s, questions: %s, sliders: %s, mentions: %s. Maximum tokens: 75, write it as simple as possible, like people would say, use simple words. Response should be in following json format: {\"description\":string,\"addIt\":bool,\"clip_length\":int}. If you think that this stories should be added to short recap video- addIt true, otherwise false . If addIt is true say what's length in seconds it should be in clip as \"clip_length\"",
							story.User.Username, temp, data, story.StoryEvents, story.StoryHashtags, story.StoryPolls, story.StoryLocations, story.StorySliders, story.StoryQuestions, story.Mentions)
					} else {
						prompt = fmt.Sprintf("I have an image from an %s's(use it when want to write about him instead of writing 'user') Instagram story. Your task is to determine if it contains any interesting or relevant information about the busines's news or sales. If it does, summarize this information in 1 short sentence. If the image content is not interesting or important activities/news, return following response: 'Nothing interesting'. Give logically connected summarize based on previous storieses(if it is empty- don't say me it is empty, give result only based on photo or return empty response):%s.Last 7 days stories: %s. Don't repeat what is already summarized and in old storieses. Additional stories info: events: %s, hashtags: %s, polls: %s, locations: %s, questions: %s, sliders: %s, mentions: %s. Maximum tokens: 75, write it as simple as possible, like people would say, use simple words. Response should be in following json format: {\"description\":string,\"addIt\":bool,\"clip_length\"}. If you think that this stories should be added to short recap video- addIt true, otherwise false. If addIt is true say what's length in seconds it should be in clip as \"clip_length\"",
							story.User.Username, temp, data, story.StoryEvents, story.StoryHashtags, story.StoryPolls, story.StoryLocations, story.StorySliders, story.StoryQuestions, story.Mentions)
					}
					resp, clip_length, addIt, err = openai.SummarizeImage(media.URL, prompt)
					used += 1
					if used >= left {
						usedIsMoreThanLeft = true
						break
					}
					if err != nil {
						logger.Error("Error summarizing image", zap.String("URL", media.URL), zap.Error(err))
						continue
					}
					if addIt {
						var tempAsset shotstack.Asset
						tempAsset.Type = "image"
						tempAsset.Src = media.URL
						tempAsset.Length = clip_length
						medias = append(medias, tempAsset)
					}

					err = redis.StoreSummarizes(context.Background(), media.URL, map[string]interface{}{"value": resp, "addIt": addIt}, "", 24*time.Hour)
					if err != nil {
						logger.Error("Error storing summarized image in Redis", zap.String("URL", media.URL), zap.Error(err))
						continue
					}

				} else {
					resp = val
				}
				if resp != "Nothing interesting" || resp != "Nothing interesting." {
					var tempStoriesType openai.StoriesType
					tempStoriesType.Author = story.User.Username
					tempStoriesType.Summarize = resp
					if profile.User.Friendship.FollowedBy {
						temp = append([]openai.StoriesType{tempStoriesType}, temp...)
					} else {
						temp = append(temp, tempStoriesType)
					}
				}
				if err = stream.Send(Format(resp)); err != nil {
					return err
				}

				break
			}

		}
		summarize, err := openai.SummarizeImagesToOne(temp, profile.User.IsBusiness, preferences)
		logger.Info(fmt.Sprintf("%s", temp))
		if err != nil {
			log.Println("Error summarizing multiple images to one for user:", username, err)
			continue
		}
		log.Println("Summarized multiple images to one:", summarize)

		if summarize != "Nothing interesting" {
			today := time.Now().Format("02.01.2006")

			todayExists := false
			for _, entry := range thisWeek {
				if inst2.EntryContainsDate(entry, today) {
					todayExists = true
					break
				}
			}
			if !todayExists {
				thisWeek = append(thisWeek, summarize+" "+today)
				if len(thisWeek) > 7 {
					thisWeek = thisWeek[len(thisWeek)-7:]
				}
				stringified, err := json.Marshal(thisWeek)
				if err != nil {
					logger.Info(fmt.Sprintf("Error marshalling this week's data for user:", username, err))
					stringified = []byte(data)
				}
				err = redis.StoreSummarizes(context.Background(), username, nil, string(stringified), 7*24*time.Hour)
				if err != nil {
					logger.Info(fmt.Sprintf("Error storing this week's data in Redis for user:", username, err))
				}
			}
			var usersStories openai.StoriesType
			usersStories.Author = username
			usersStories.Summarize = summarize
			storiesArray = append(storiesArray, usersStories)
		}
	}

	jsoned, err := json.Marshal(storiesArray)
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling stories array:", storiesArray, err))
	}
	if !isDaily {
		return stream.Send(&grpc.SummarizeStoriesResponse{Result: string(jsoned), LinkToVideo: "", Used: used})
	}
	skip := false
	Data, err := shotstack.GenerateVideoJson(medias)
	if err != nil {
		logger.Error(fmt.Sprintf("Error generating video JSON from medias:", err))
		skip = true
	}
	logger.Info(fmt.Sprintf("Generated video JSON from medias:", data))
	var Id string
	if !skip {
		Id, err = shotstack.GenerateVideo(Data)
		if err != nil {
			logger.Error(fmt.Sprintf("Error generating video ID:", err))
			skip = true
		}
	}
	logger.Info(fmt.Sprintf("Generated video with ID:", Id))
	var url string
	if !skip {
		url, err = shotstack.GetUrl(Id)
		if err != nil {
			logger.Error(fmt.Sprintf("Error generating video URL:", err))
			skip = true
		}
	}
	return stream.Send(&grpc.SummarizeStoriesResponse{Result: string(jsoned), LinkToVideo: url, Used: used})
	return nil
}

func Format(text string) *grpc.SummarizeStoriesResponse {
	return &grpc.SummarizeStoriesResponse{Result: text}
}

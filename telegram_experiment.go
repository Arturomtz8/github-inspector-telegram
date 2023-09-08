// This is the function that is called by google functions,
// the structure of the code must be like specified in the
// docs https://cloud.google.com/functions/docs/writing#directory-structure

package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/Arturomtz8/github-inspector/pkg/github"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

const (
	searchCommand          string = "/search"
	telegramApiBaseUrl     string = "https://api.telegram.org/bot"
	telegramApiSendMessage string = "/sendMessage"
	telegramTokenEnv       string = "GITHUB_BOT_TOKEN"
	defaulRepoLen          int    = 10
)

var lenSearchCommand int = len(searchCommand)

// Chat struct stores the id of the chat in question.
type Chat struct {
	Id int `json:"id"`
}

// Message struct store Chat and text data.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// trigger deploy
// Update event.
type Update struct {
	UpdateId int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Register an HTTP function with the Functions Framework
func init() {
	functions.HTTP("HandleTelegramWebhook", HandleTelegramWebhook)
}

// HandleTelegramWebhook is the web hook that has to have the handler signature.
// Listen for incoming web requests from Telegram events and
// responds back with the treding repositories on GitHub.
func HandleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	var update, err = parseTelegramRequest(r)
	if err != nil {
		fmt.Printf("error parsing update, %s", err.Error())
		return
	}

	sanitizedString, err := sanitize(update.Message.Text)
	if err != nil {
		sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
		fmt.Fprintf(w, "Invald input")
		return
	}
	fmt.Println("Sanitized string is: ", sanitizedString)
	repos, err := github.GetTrendingRepos(github.TimeToday, sanitizedString)
	if err != nil {
		sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
		fmt.Fprintf(w, "An error has ocurred, %s!", err)
		return
	}
	fmt.Println("raw repos: ", repos)
	responseFunc, err := formatReposContentAndSend(repos, update.Message.Chat.Id)
	if err != nil {
		sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
		fmt.Printf("got error %s from parsing repos", err.Error())
		return

	} else {
		fmt.Printf("successfully distributed to chat id %d, response from loop: %s", update.Message.Chat.Id, responseFunc)
		return
	}

}

// parseTelegramRequest decodes and incoming request from the Telegram hook,
// and returns an Update pointer.
func parseTelegramRequest(r *http.Request) (*Update, error) {
	var update Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		fmt.Printf("could not decode incoming update %s", err.Error())
		return nil, err
	}
	return &update, nil
}

// returns the term that wants to be searched or
// an string that specifies the expected input
func sanitize(s string) (string, error) {
	if len(s) >= lenSearchCommand {
		if s[:lenSearchCommand] == searchCommand {
			s = s[lenSearchCommand:]
			s = strings.TrimSpace(s)
			fmt.Printf("type of value entered: %T\n", s)
		} else {
			return "", errors.New("invalid value: you must enter /search {languague}")
		}

	} else {
		return "", errors.New("invalid value: you must enter /search {languague}")
	}
	return s, nil

}

// Formats the content of the repos and uses internally sendTextToTelegramChat function
// for sending the formatted content to the respective chat
func formatReposContentAndSend(repos *github.TrendingSearchResult, chatId int) (string, error) {
	var repoLen int
	reposContent := make([]string, 0)

	const templ = `
	{{.FullName}}: {{.Description}}
	Author: {{.Owner.Login}}
	⭐: {{.StargazersCount}}
	{{.HtmlURL}}
	`

	for _, repo := range repos.Items {
		var report = template.Must(template.New("trendinglist").Parse(templ))
		buf := &bytes.Buffer{}
		if err := report.Execute(buf, repo); err != nil {
			sendTextToTelegramChat(chatId, err.Error())
			return "", err
		}
		s := buf.String()

		reposContent = append(reposContent, s)
	}

	if len(reposContent) == 0 {
		return "", errors.New("There are not trending repos yet for today, try again later")

	} else if len(reposContent) <= defaulRepoLen {
		repoLen = len(reposContent)
	} else {
		repoLen = defaulRepoLen
	}
	fmt.Println("template created and proceeding to send repos to chat")
	fmt.Println("Total repos that will be sent", repoLen)
	for i := 0; i < repoLen; i++ {

		repo := reposContent[i]
		if _, err := sendTextToTelegramChat(chatId, repo); err != nil {
			// No need to break loop, just continue to the next one.
			fmt.Printf("error occurred publishing event %v", err)
			continue
		}

	}
	return "all repos sent to chat", nil
}

// sendTextToTelegramChat sends the response from the GitHub back to the chat,
// given a chat id and the text from GitHub.
func sendTextToTelegramChat(chatId int, text string) (string, error) {
	fmt.Printf("Sending %s to chat_id: %d", text, chatId)

	var telegramApi string = "https://api.telegram.org/bot" + os.Getenv("GITHUB_BOT_TOKEN") + "/sendMessage"

	response, err := http.PostForm(
		telegramApi,
		url.Values{
			"chat_id": {strconv.Itoa(chatId)},
			"text":    {text},
		})
	if err != nil {
		fmt.Printf("error when posting text to the chat: %s", err.Error())
		return "", err
	}
	defer response.Body.Close()
	var bodyBytes, errRead = ioutil.ReadAll(response.Body)
	if errRead != nil {
		fmt.Printf("error parsing telegram answer %s", errRead.Error())
		return "", err
	}

	bodyString := string(bodyBytes)
	fmt.Printf("body of telegram response: %s", bodyString)
	return bodyString, nil

}
